# Security Policy

## Supported Versions

We release security fixes for the latest stable version of `breq`.

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |
| Older   | No        |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Please report security issues by emailing **security@bluefunda.com** with:

- A description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested mitigations

We will acknowledge your report within 48 hours and aim to release a fix within 7 days for critical issues.

## Security Practices

- All releases are signed and include SHA256 checksums
- macOS binaries are notarized by Apple
- Tokens are stored in OS-specific credential stores, never in plaintext config files
- gRPC connections use TLS by default
