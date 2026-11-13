package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
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

	// Split the line into request type, requested resource and HTTP version.
	parts := strings.Fields(requestLine)
	if len(parts) != 3 {
		return fmt.Errorf("invalid HTTP request line: %q", requestLine)
	}

	method := parts[0]
	path := parts[1]
	version := parts[2]

	fmt.Printf("Request Type: %s, Requested Resource: %s, Version: %s\n", method, path, version)

	// Construct a minimal HTTP response.
	response := fmt.Sprintf("HTTP/1.1 200 OK\r\n\r\nRequested resource: %s\r\n", path)

	// Send the HTTP response to the client.
	_, err = conn.Write([]byte(response))
	if err != nil {
		return fmt.Errorf("write HTTP response: %w", err)
	}

	return nil
}
