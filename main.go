package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Create a TCP listener on localhost, port 80.
	listener, err := net.Listen("tcp", "127.0.0.1:80")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Println("Server listening on http://127.0.0.1:80")

	for {
		// Wait for an incoming TCP connection.
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}

		// Handle each TCP connection concurrently.
		go func() {
			if err := handleConnection(conn); err != nil {
				log.Println("Error handling connection:", err)
			}
		}()
	}
}

// handleConnection reads an HTTP request and serves the requested file.
func handleConnection(conn net.Conn) error {
	// Close the TCP connection when finished.
	defer conn.Close()

	// Set a deadline for receiving the HTTP request.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	// Create a buffered reader for the incoming HTTP request.
	reader := bufio.NewReader(conn)

	// Read the first line of the HTTP request.
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read HTTP request line: %w", err)
	}

	// Extract the request method, path to requested resource and HTTP version.
	parts := strings.Fields(requestLine)
	if len(parts) != 3 {
		return writeResponse(conn, "400 Bad Request", nil, nil)
	}

	requestMethod := parts[0]
	requestPath := parts[1]
	version := parts[2]

	// Validate the request method.
	if requestMethod != "GET" {
		return writeResponse(conn, "405 Method Not Allowed", nil, nil)
	}

	// Validate the request path.
	if !strings.HasPrefix(requestPath, "/") {
		return writeResponse(conn, "400 Bad Request", nil, nil)
	}

	// Validate the HTTP version.
	if version != "HTTP/1.1" {
		return writeResponse(conn, "505 HTTP Version Not Supported", nil, nil)
	}

	fmt.Printf("Request Method: %s, Requested Resource: %s, HTTP Version: %s\n", requestMethod, requestPath, version)

	// Separate the path from the query if one is present.
	requestPath, query, _ := strings.Cut(requestPath, "?")

	// Serve 'index.html' for the root path '/'.
	if requestPath == "/" {
		requestPath = "/index.html"
	}

	// Check whether the path is for a CGI script.
	isCGI := strings.HasPrefix(requestPath, "/cgi-bin/")

	// Convert the path into a relative path.
	relativePath := strings.TrimPrefix(requestPath, "/")
	if isCGI {
		relativePath = strings.TrimPrefix(relativePath, "cgi-bin/")
	}

	// Validate and localise the path.
	localPath, err := filepath.Localize(relativePath)
	if err != nil {
		return writeResponse(conn, "403 Forbidden", nil, nil)
	}

	// Execute the requested file if it is a CGI script.
	if isCGI {
		return handleCGI(conn, localPath, requestPath, query)
	}

	// Serve the requested file if it is a static file.
	return handleStatic(conn, localPath)
}

// handleStatic reads a static file and sends it to the client.
func handleStatic(conn net.Conn, localPath string) error {
	// Open the document root to prevent access outside the www directory.
	root, err := os.OpenRoot("www")
	if err != nil {
		return handleInternalError(conn, "500 Internal Server Error", fmt.Errorf("open document root: %w", err))
	}
	defer root.Close()

	// Read the static file.
	content, err := root.ReadFile(localPath)
	if err != nil {
		if os.IsPermission(err) {
			return writeResponse(conn, "403 Forbidden", nil, nil)
		}
		if os.IsNotExist(err) {
			return writeResponse(conn, "404 Not Found", nil, nil)
		}
		return handleInternalError(conn, "500 Internal Server Error", fmt.Errorf("read file: %w", err))
	}

	// Determine the file's content type.
	contentType := mime.TypeByExtension(filepath.Ext(localPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Construct the Content-Type header.
	headers := map[string]string{
		"Content-Type": contentType,
	}

	// Send the static file.
	return writeResponse(conn, "200 OK", headers, content)
}

// handleCGI executes a CGI script and sends its output to the client.
func handleCGI(conn net.Conn, localPath, requestPath, query string) error {
	// Open the document root to prevent access outside the cgi-bin directory.
	root, err := os.OpenRoot("cgi-bin")
	if err != nil {
		return handleInternalError(conn, "500 Internal Server Error", fmt.Errorf("open document root: %w", err))
	}
	defer root.Close()

	// Validate the CGI script.
	if filepath.Ext(localPath) != ".pl" {
		return writeResponse(conn, "403 Forbidden", nil, nil)
	}

	// Read the CGI script.
	script, err := root.ReadFile(localPath)
	if err != nil {
		if os.IsPermission(err) {
			return writeResponse(conn, "403 Forbidden", nil, nil)
		}
		if os.IsNotExist(err) {
			return writeResponse(conn, "404 Not Found", nil, nil)
		}
		return handleInternalError(conn, "500 Internal Server Error", fmt.Errorf("read script: %w", err))
	}

	// Limit script execution to 30 seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute the CGI script using Perl.
	cmd := exec.CommandContext(ctx, "perl", "-")
	cmd.Stdin = bytes.NewReader(script)
	cmd.Env = append(os.Environ(),
		"GATEWAY_INTERFACE=CGI/1.1",
		"QUERY_STRING="+query,
		"REQUEST_METHOD=GET",
		"SCRIPT_NAME="+requestPath,
		"SERVER_NAME=localhost",
		"SERVER_PORT=80",
		"SERVER_PROTOCOL=HTTP/1.1",
	)

	// Store the CGI script's output.
	output, err := cmd.Output()
	if err != nil {
		status := "500 Internal Server Error"
		if ctx.Err() == context.DeadlineExceeded {
			status = "504 Gateway Timeout"
		}
		return handleInternalError(conn, status, fmt.Errorf("execute script: %w", err))
	}

	// Parse the CGI script's output.
	status, headers, body, err := parseCGIResponse(output)
	if err != nil {
		return handleInternalError(conn, "500 Internal Server Error", err)
	}

	// Send the CGI script's output.
	return writeResponse(conn, status, headers, body)
}

// parseCGIResponse extracts the status, headers and body from a CGI script's output.
func parseCGIResponse(output []byte) (string, map[string]string, []byte, error) {
	// Separate the CGI response headers from the CGI response body.
	header, body, found := bytes.Cut(output, []byte("\r\n\r\n"))
	if !found {
		header, body, found = bytes.Cut(output, []byte("\n\n"))
	}
	if !found {
		return "", nil, nil, fmt.Errorf("invalid CGI response: missing separator")
	}

	// Initialise the CGI response headers and default status.
	headers := make(map[string]string)
	status := "200 OK"

	// Parse the CGI response headers.
	for _, line := range strings.Split(strings.ReplaceAll(string(header), "\r\n", "\n"), "\n") {
		// Separate the header name from the header value.
		name, value, found := strings.Cut(line, ":")
		if !found {
			return "", nil, nil, fmt.Errorf("invalid CGI header: %q", line)
		}

		// Normalise the header name and trim whitespace.
		name = http.CanonicalHeaderKey(strings.TrimSpace(name))
		value = strings.TrimSpace(value)

		// Reject an empty header name.
		if name == "" {
			return "", nil, nil, fmt.Errorf("invalid CGI header: %q", line)
		}

		// Validate the header name.
		const tokenChars = "!#$%&'*+-.^_`|~"
		for _, r := range name {
			if ('A' <= r && r <= 'Z') ||
				('a' <= r && r <= 'z') ||
				('0' <= r && r <= '9') ||
				strings.ContainsRune(tokenChars, r) {
				continue
			}
			return "", nil, nil, fmt.Errorf("invalid CGI header: %q", line)
		}

		// Validate the header value.
		for _, r := range value {
			if r < 32 && r != '\t' || r == 127 {
				return "", nil, nil, fmt.Errorf("invalid CGI header: %q", line)
			}
		}

		// Extract and validate the CGI status.
		if name == "Status" {
			codeString, _, found := strings.Cut(value, " ")
			code, err := strconv.Atoi(codeString)
			if !found || err != nil || len(codeString) != 3 || code < 200 || 599 < code {
				return "", nil, nil, fmt.Errorf("invalid CGI status: %q", value)
			}

			switch code {
			case 204, 205, 304:
				return "", nil, nil, fmt.Errorf("unsupported CGI status: %d", code)
			}

			reason := http.StatusText(code)
			if reason == "" {
				return "", nil, nil, fmt.Errorf("unknown CGI status code: %d", code)
			}

			status = fmt.Sprintf("%d %s", code, reason)
		}

		// Skip headers managed by the server.
		switch name {
		case "Status", "Content-Length", "Date", "Connection", "Transfer-Encoding":
			continue
		default:
			headers[name] = value
		}
	}

	// Validate that the Content-Type header is present and non-empty.
	if headers["Content-Type"] == "" {
		return "", nil, nil, fmt.Errorf("missing CGI header: Content-Type")
	}

	return status, headers, body, nil
}

// handleInternalError sends an HTTP error response and returns the original error.
func handleInternalError(conn net.Conn, status string, err error) error {
	if writeErr := writeResponse(conn, status, nil, nil); writeErr != nil {
		return writeErr
	}
	return err
}

// writeResponse sends an HTTP response with the given status, headers and body.
func writeResponse(conn net.Conn, status string, headers map[string]string, body []byte) error {
	// Set a deadline for sending the HTTP response.
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	// Initialise the headers if necessary.
	if headers == nil {
		headers = make(map[string]string)
	}

	// Construct the server-controlled headers.
	headers["Content-Length"] = strconv.Itoa(len(body))
	headers["Date"] = time.Now().UTC().Format(http.TimeFormat)
	headers["Connection"] = "close"

	// Construct the HTTP response.
	var response bytes.Buffer
	fmt.Fprintf(&response, "HTTP/1.1 %s\r\n", status)

	for name, value := range headers {
		fmt.Fprintf(&response, "%s: %s\r\n", name, value)
	}

	response.WriteString("\r\n")
	response.Write(body)

	// Write the HTTP response to the TCP connection.
	if _, err := conn.Write(response.Bytes()); err != nil {
		return fmt.Errorf("write HTTP response: %w", err)
	}

	return nil
}
