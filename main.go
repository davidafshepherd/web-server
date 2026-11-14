package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
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

		// Handle one TCP connection at a time.
		if err := handleConnection(conn); err != nil {
			log.Println("Error handling connection:", err)
		}
	}
}

// handleConnection reads an HTTP request and serves the requested file.
func handleConnection(conn net.Conn) error {
	// Close the TCP connection when finished.
	defer conn.Close()

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
		return fmt.Errorf("invalid HTTP request line: %q", requestLine)
	}

	requestMethod := parts[0]
	requestPath := parts[1]
	version := parts[2]

	fmt.Printf("Request Method: %s, Requested Resource: %s, HTTP Version: %s\n", requestMethod, requestPath, version)

	// Remove any query parameters from the path.
	requestPath = strings.SplitN(requestPath, "?", 2)[0]

	// Serve 'index.html' for the root path '/'.
	if requestPath == "/" {
		requestPath = "/index.html"
	}

	// Convert the path into a relative path.
	relativePath := strings.TrimPrefix(requestPath, "/")

	// Validate the path and convert it into a local filesystem path.
	localPath, err := filepath.Localize(relativePath)
	if err != nil {
		return writeResponse(conn, "403 Forbidden", nil)
	}

	// Resolve the requested file inside the www directory.
	filePath := filepath.Join("www", localPath)

	// Read the requested file.
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return writeResponse(conn, "404 Not Found", nil)
		}
		return fmt.Errorf("read file: %w", err)
	}

	// Send the requested file.
	return writeResponse(conn, "200 OK", content)
}

// writeResponse sends an HTTP response with the given status and body.
func writeResponse(conn net.Conn, status string, body []byte) error {
	// Construct the HTTP response.
	response := fmt.Sprintf("HTTP/1.1 %s\r\n\r\n", status)

	// Write the HTTP response to the TCP connection.
	if _, err := conn.Write(append([]byte(response), body...)); err != nil {
		return fmt.Errorf("write HTTP response: %w", err)
	}

	return nil
}
