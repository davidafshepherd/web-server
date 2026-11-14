package main

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
)

func TestHandleConnection(t *testing.T) {
	// Construct the tests.
	tests := []struct {
		name           string
		request        string
		expectedStatus string
		expectedBody   string
		wantErr        bool
	}{
		{
			name:    "invalid request line",
			request: "INVALID\r\n",
			wantErr: true,
		},
		{
			name:           "root path",
			request:        "GET / HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedBody:   "My web server served this page!",
		},
		{
			name:           "existing file",
			request:        "GET /index.html HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedBody:   "My web server served this page!",
		},
		{
			name:           "missing file",
			request:        "GET /missing.html HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 404 Not Found",
		},
		{
			name:           "path traversal",
			request:        "GET /../go.mod HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 403 Forbidden",
		},
		{
			name:           "query parameters",
			request:        "GET /index.html?test=123 HTTP/1.1\r\n\r\n",
			expectedStatus: "HTTP/1.1 200 OK",
			expectedBody:   "My web server served this page!",
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

			// Ensure the connection handler completed as expected.
			err = <-done

			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Read the HTTP response status line.
			reader := bufio.NewReader(strings.NewReader(string(response)))
			status, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}

			// Validate that the HTTP response contains the expected status.
			if strings.TrimSpace(status) != test.expectedStatus {
				t.Errorf("expected %q, got %q", test.expectedStatus, strings.TrimSpace(status))
			}

			// Validate that the HTTP response contains the expected body.
			if test.expectedBody != "" && !strings.Contains(string(response), test.expectedBody) {
				t.Errorf("expected response to contain %q, got %q", test.expectedBody, string(response))
			}
		})
	}
}
