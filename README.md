# Phite CLI

Phite CLI is an open-source local development environment for PHP Projects. It is designed to make a Project locally runnable without requiring the Developer to assemble a machine-wide PHP and web-server stack.

Phite is currently pre-alpha. The first implemented command reports build information:

```console
go run ./cmd/phite version
```

## Development

Phite requires Go 1.26.7 or newer within the Go 1.26 release line.

Run the complete test suite through the public CLI process seam:

```console
go test ./...
```

## License

Phite CLI is licensed under the Apache License 2.0. Third-party Managed Runtime artifacts retain their own licenses and notices.
