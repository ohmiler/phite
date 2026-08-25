# Phite CLI

Phite CLI is an open-source local development environment for PHP Projects. It is designed to make a Project locally runnable without requiring the Developer to assemble a machine-wide PHP and web-server stack.

Phite is currently pre-alpha. On a supported platform, the CLI can report its build and run PHP through a pinned Managed Runtime:

```console
go run ./cmd/phite version
go run ./cmd/phite php -v
```

The first `phite php` invocation downloads the Runtime Identity embedded in the CLI, verifies its SHA-256 checksum before extraction, and stores it in the Developer-scoped Runtime Cache. Later invocations reuse the verified cache and do not require a system PHP installation.

## Development

Phite requires Go 1.26.7 or newer within the Go 1.26 release line.

Run the complete test suite through the public CLI process seam:

```console
go test ./...
```

## License

Phite CLI is licensed under the Apache License 2.0. Third-party Managed Runtime artifacts retain their own licenses and notices.
