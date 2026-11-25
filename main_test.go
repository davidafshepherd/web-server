package main

import (
	"bufio"
	"bytes"
	"io"
	"maps"
	"net"
	"net/http"
	"testing"
)

func TestHandleConnection(t *testing.T) {
	// Construct the tests.
	tests := []struct {
		name            string
		request         string
		expectedStatus  string
		expectedHeaders map[string]string
		expectedBody    string
	}{
		{
			name:           "invalid request line",
			request:        "INVALID\r\n",
			expectedStatus: "HTTP/1.1 400 Bad Request",
		},
		{
			name:           "unsupported HTTP method",
			request:        "POST /index.html HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 405 Method Not Allowed",
		},
		{
			name:           "unsupported HTTP version",
			request:        "GET /index.html HTTP/1.0\r\n\r\n",
			expectedStatus: "HTTP/1.1 505 HTTP Version Not Supported",
		},
		{
			name:           "root path",
			request:        "GET / HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/html; charset=utf-8",
			},
			expectedBody: "<!DOCTYPE html>\r\n<html lang=\"en\">\r\n  <head>\r\n    <title>Simple Web Page</title>\r\n  </head>\r\n  <body>\r\n    <h1>Test Web Page</h1>\r\n    <p>My web server served this page!</p>\r\n  </body>\r\n</html>\r\n",
		},
		{
			name:           "path traversal",
			request:        "GET /../go.mod HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 403 Forbidden",
		},
		{
			name:           "existing static file",
			request:        "GET /index.html HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/html; charset=utf-8",
			},
			expectedBody: "<!DOCTYPE html>\r\n<html lang=\"en\">\r\n  <head>\r\n    <title>Simple Web Page</title>\r\n  </head>\r\n  <body>\r\n    <h1>Test Web Page</h1>\r\n    <p>My web server served this page!</p>\r\n  </body>\r\n</html>\r\n",
		},
		{
			name:           "existing static file with query parameters",
			request:        "GET /index.html?test=123 HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedBody:   "<!DOCTYPE html>\r\n<html lang=\"en\">\r\n  <head>\r\n    <title>Simple Web Page</title>\r\n  </head>\r\n  <body>\r\n    <h1>Test Web Page</h1>\r\n    <p>My web server served this page!</p>\r\n  </body>\r\n</html>\r\n",
		},
		{
			name:           "missing static file",
			request:        "GET /missing.html HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 404 Not Found",
		},
		{
			name:           "existing CGI script",
			request:        "GET /cgi-bin/hello.pl HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/html",
			},
			expectedBody: "<h1>Hello from Perl CGI!</h1>\r\n<p>Method: GET</p>\r\n<p>Query: </p>\r\n",
		},
		{
			name:           "existing CGI script with query parameters",
			request:        "GET /cgi-bin/hello.pl?name=David HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/html",
			},
			expectedBody: "<h1>Hello from Perl CGI!</h1>\r\n<p>Method: GET</p>\r\n<p>Query: name=David</p>\r\n",
		},
		{
			name:           "missing CGI script",
			request:        "GET /cgi-bin/missing.pl HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 404 Not Found",
		},
		{
			name:           "unsupported CGI script extension",
			request:        "GET /cgi-bin/hello.py HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 403 Forbidden",
		},
	}

	// Run the tests.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create an in-memory TCP-like connection between a server and a client.
			server, client := net.Pipe()
			defer client.Close()

			// Handle the server-side connection.
			done := make(chan error, 1)
			go func() {
				done <- handleConnection(server)
			}()

			// Send the HTTP request.
			if _, err := io.WriteString(client, test.request); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Read the HTTP response.
			response, err := io.ReadAll(client)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Ensure the connection handler completed successfully.
			if err := <-done; err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Parse the HTTP response.
			reader := bufio.NewReader(bytes.NewReader(response))
			resp, err := http.ReadResponse(reader, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate the HTTP response status.
			status := resp.Proto + " " + resp.Status
			if status != test.expectedStatus {
				t.Errorf("Status: expected %q, got %q", test.expectedStatus, status)
			}

			// Validate the HTTP response headers.
			for header, expected := range test.expectedHeaders {
				if got := resp.Header.Get(header); got != expected {
					t.Errorf("%q Header: expected to be %q, got %q", header, expected, got)
				}
			}

			// Read the HTTP response body.
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate the Content-Length header.
			if resp.ContentLength != int64(len(body)) {
				t.Errorf("Content-Length Header: expected %d, got %d", len(body), resp.ContentLength)
			}

			// Validate the Date header.
			if _, err := http.ParseTime(resp.Header.Get("Date")); err != nil {
				t.Errorf("invalid Date Header: %v", err)
			}

			// Validate the Connection header.
			if !resp.Close {
				t.Error("invalid Connection Header")
			}

			// Validate the HTTP response body.
			if string(body) != test.expectedBody {
				t.Errorf("Response Body: expected %q, got %q", test.expectedBody, string(body))
			}

			// Validate that no additional response data was sent.
			if _, err := reader.Peek(1); err != io.EOF {
				t.Errorf("expected EOF after HTTP response, got %v", err)
			}
		})
	}
}

func TestParseCGIResponse(t *testing.T) {
	// Construct the tests.
	tests := []struct {
		name            string
		cgiOutput       string
		expectedStatus  string
		expectedHeaders map[string]string
		expectedBody    string
		wantErr         bool
	}{
		{
			name:           "valid CGI response with CRLF",
			cgiOutput:      "Content-Type: text/html\r\n\r\nHello",
			expectedStatus: "200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/html",
			},
			expectedBody: "Hello",
		},
		{
			name:           "valid CGI response with LF",
			cgiOutput:      "Content-Type: text/plain\n\nHello",
			expectedStatus: "200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/plain",
			},
			expectedBody: "Hello",
		},
		{
			name:           "valid CGI response with Status header",
			cgiOutput:      "Status: 201 Created\nContent-Type: text/plain\n\nCreated",
			expectedStatus: "201 Created",
			expectedHeaders: map[string]string{
				"Content-Type": "text/plain",
			},
			expectedBody: "Created",
		},
		{
			name:           "valid CGI response with non-standard status reason",
			cgiOutput:      "Status: 202 Not OK\nContent-Type: text/plain\n\nHello",
			expectedStatus: "202 Accepted",
			expectedHeaders: map[string]string{
				"Content-Type": "text/plain",
			},
			expectedBody: "Hello",
		},
		{
			name:           "valid CGI response with case-insensitive header name",
			cgiOutput:      "content-type: text/plain\n\nHello",
			expectedStatus: "200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/plain",
			},
			expectedBody: "Hello",
		},
		{
			name:           "valid CGI response with server-controlled headers",
			cgiOutput:      "Content-Type: text/plain\nContent-Length: 100\nDate: invalid\nConnection: keep-alive\n\nHello",
			expectedStatus: "200 OK",
			expectedHeaders: map[string]string{
				"Content-Type": "text/plain",
			},
			expectedBody: "Hello",
		},
		{
			name:           "valid CGI response with server error status and a body",
			cgiOutput:      "Status: 503 Service Unavailable\nContent-Type: text/plain\n\nTry again later",
			expectedStatus: "503 Service Unavailable",
			expectedHeaders: map[string]string{
				"Content-Type": "text/plain",
			},
			expectedBody: "Try again later",
		},
		{
			name:      "missing separator",
			cgiOutput: "Content-Type: text/html",
			wantErr:   true,
		},
		{
			name:      "informational status",
			cgiOutput: "Status: 100 Continue\nContent-Type: text/plain\n\nHello",
			wantErr:   true,
		},
		{
			name:      "no content status",
			cgiOutput: "Status: 204 No Content\nContent-Type: text/plain\n\n",
			wantErr:   true,
		},
		{
			name:      "reset content status",
			cgiOutput: "Status: 205 Reset Content\nContent-Type: text/plain\n\n",
			wantErr:   true,
		},
		{
			name:      "not modified status",
			cgiOutput: "Status: 304 Not Modified\nContent-Type: text/plain\n\n",
			wantErr:   true,
		},
		{
			name:      "unknown status code",
			cgiOutput: "Status: 299 Unknown\nContent-Type: text/plain\n\nHello",
			wantErr:   true,
		},
		{
			name:      "out of range status code",
			cgiOutput: "Status: 600 Invalid\nContent-Type: text/plain\n\nHello",
			wantErr:   true,
		},
		{
			name:      "invalid status code length",
			cgiOutput: "Status: 0200 OK\nContent-Type: text/plain\n\nHello",
			wantErr:   true,
		},
		{
			name:      "non-numeric status code",
			cgiOutput: "Status: abc Invalid\nContent-Type: text/plain\n\nHello",
			wantErr:   true,
		},
		{
			name:      "missing status reason",
			cgiOutput: "Status: 202\nContent-Type: text/plain\n\nHello",
			wantErr:   true,
		},
		{
			name:      "empty Content-Type header",
			cgiOutput: "Content-Type:\n\nHello",
			wantErr:   true,
		},
		{
			name:      "missing Content-Type header",
			cgiOutput: "X-Custom-Header: test\n\nHello",
			wantErr:   true,
		},
		{
			name:      "malformed header",
			cgiOutput: "Content-Type text/html\n\nHello",
			wantErr:   true,
		},
		{
			name:      "invalid header name",
			cgiOutput: "Content-Type: text/plain\nInvalid Header: test\n\nHello",
			wantErr:   true,
		},
		{
			name:      "invalid header value",
			cgiOutput: "Content-Type: text/plain\x00\n\nHello",
			wantErr:   true,
		},
	}

	// Run the tests.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, headers, body, err := parseCGIResponse([]byte(test.cgiOutput))

			if test.wantErr {
				if err == nil {
					t.Error("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate the CGI response status.
			if status != test.expectedStatus {
				t.Errorf("Status: expected %q, got %q", test.expectedStatus, status)
			}

			// Validate the CGI response headers.
			if !maps.Equal(headers, test.expectedHeaders) {
				t.Errorf("Headers: expected %v, got %v", test.expectedHeaders, headers)
			}

			// Validate the CGI response body.
			if string(body) != test.expectedBody {
				t.Errorf("Response Body: expected %q, got %q", test.expectedBody, string(body))
			}
		})
	}
}
