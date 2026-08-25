# Phite Managed Runtime notices: Windows x64

Runtime Identity: `frankenphp-1.12.7-php-8.5.9-windows-x64`

Phite redistributes the unmodified `frankenphp-windows-x86_64.zip` asset from
the official FrankenPHP v1.12.7 release. Its verified SHA-256 digest is:

`edec8d3c43508f98b498af911f47aa93ebc51f7e46f1d26a9d41adb7ccbaa828`

The runtime reports FrankenPHP 1.12.7, PHP 8.5.9, and Caddy 2.11.4. Third-party
software remains governed by its own license; the Apache-2.0 license for Phite
CLI does not replace those terms.

The notice bundle contains:

- the license inventory for the Go packages linked into FrankenPHP;
- the corresponding license and notice files collected from their pinned
  module versions;
- PHP's `license.txt`, `readme-redist-bins.txt`, and CycloneDX, OpenVEX, and
  SPDX SBOM documents copied from the verified upstream artifact; and
- the full AGPL-3.0 license text for the Mercure and Vulcain modules.

Corresponding source for the pinned runtime and its copyleft modules is
available without charge from these exact upstream revisions:

- FrankenPHP v1.12.7 (`a765b086f5cc56f6b7753117367d56e1b0da948d`):
  https://github.com/php/frankenphp/tree/v1.12.7
- PHP 8.5.9:
  https://github.com/php/php-src/tree/php-8.5.9
- Caddy v2.11.4:
  https://github.com/caddyserver/caddy/tree/v2.11.4
- Mercure v0.24.2:
  https://github.com/dunglas/mercure/tree/v0.24.2
- Vulcain v1.4.2:
  https://github.com/dunglas/vulcain/tree/v1.4.2

The complete module versions and versioned license locations are recorded in
`go-licenses.csv`. PHP's bundled native libraries and their notices are listed
in the files copied from the upstream PHP distribution under `artifact/`.
