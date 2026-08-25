# Phite CLI

Phite CLI is an open-source local development environment for PHP Projects. It is designed to make a Project locally runnable without requiring the Developer to assemble a machine-wide PHP, Composer, and web-server stack.

Phite is currently pre-alpha. On a supported platform, the CLI can report its build, run PHP through a pinned Managed Runtime, and run a pinned Composer PHAR through that same runtime:

```console
go run ./cmd/phite version
go run ./cmd/phite php -v
go run ./cmd/phite composer install
```

The first `phite php` invocation downloads the Runtime Identity embedded in the CLI, verifies its SHA-256 checksum before extraction, and stores it in the Developer-scoped Runtime Cache. Later invocations reuse the verified cache and do not require a system PHP installation.

The first `phite composer` invocation acquires that same Managed Runtime and the Composer version embedded in the CLI. Phite verifies the Composer PHAR before execution, stores it in a separate Developer-scoped cache, and preserves Composer's arguments, Project working directory, standard streams, and exit status. Neither a system PHP nor a globally installed Composer is used.

## Development

Phite requires Go 1.26.7 or newer within the Go 1.26 release line.

Run the complete test suite through the public CLI process seam:

```console
go test ./...
```

## License

Phite CLI is licensed under the Apache License 2.0. Third-party Managed Runtime and Composer artifacts retain their own licenses and notices.
