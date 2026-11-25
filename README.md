# Web Server

- [main.go](main.go) contains the web server implementation.
- [www](www) contains the static files served by the web server.
- [cgi-bin](cgi-bin) contains the Perl CGI scripts executed by the web server.
#
To start the web server, run:

```bash
go run .
```

The web server listens on `http://127.0.0.1:80`, handles concurrent TCP connections and supports HTTP/1.1 GET requests. It serves static files from the `www` directory and executes Perl CGI scripts from the `cgi-bin` directory.

To request a static file, run:

```bash
curl "http://127.0.0.1/<file_path>"
```

To execute a CGI script, run:

```bash
curl "http://127.0.0.1/cgi-bin/<script_path>"
```

To execute a CGI script with a query string, run:

```bash
curl "http://127.0.0.1/cgi-bin/<script_path>?<query_string>"
```

To run the tests, run:

```bash
go test ./...
```

This project requires **Go 1.27.3 or later** and **Perl** (tested with **v5.42.3**).
