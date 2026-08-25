package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	listen, root, err := arguments(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if os.Getenv("FAKE_FRANKENPHP_NEVER_READY") == "1" {
		if os.Getenv("FAKE_FRANKENPHP_IGNORE_TERMINATION") == "1" {
			ignoreTermination()
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	fileServer := http.FileServer(http.Dir(root))
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if marker := os.Getenv("FAKE_FRANKENPHP_REQUEST_MARKER"); marker != "" {
			_ = os.WriteFile(marker, []byte(request.URL.Path), 0o600)
		}
		path := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(request.URL.Path, "/")))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(response, request)
			return
		}
		_, _ = fmt.Fprintf(response, "front controller:%s", request.URL.Path)
	})
	if err := http.ListenAndServe(listen, handler); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func arguments(values []string) (string, string, error) {
	if len(values) == 0 || values[0] != "php-server" {
		return "", "", fmt.Errorf("expected php-server command")
	}
	var listen, root string
	for _, value := range values[1:] {
		if parsed, ok := strings.CutPrefix(value, "--listen="); ok {
			listen = parsed
		}
		if parsed, ok := strings.CutPrefix(value, "--root="); ok {
			root = parsed
		}
	}
	if listen == "" || root == "" {
		return "", "", fmt.Errorf("missing --listen or --root")
	}
	return listen, root, nil
}
