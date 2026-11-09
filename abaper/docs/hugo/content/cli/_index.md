---
title: "CLI Reference"
description: "Complete command line interface reference for abaper"
weight: 100
date: 2025-08-01T03:31:55Z
lastmod: 2025-08-01T03:31:55Z
draft: false
---

# ABAPER CLI Reference

This section contains the complete command line interface reference for the ABAPER tool.

ABAPER is a comprehensive CLI tool for interacting with SAP ABAP systems via ADT (ABAP Development Tools). It supports retrieving source code, searching objects, testing connections, and running as a REST API server.

## Available Commands

- **[abaper](abaper/)** - Main CLI tool with global options
- **[abaper get](abaper_get/)** - Retrieve ABAP object source code
- **[abaper search](abaper_search/)** - Search for ABAP objects
- **[abaper list](abaper_list/)** - List objects of specified type
- **[abaper connect](abaper_connect/)** - Test ADT connection
- **[abaper server](abaper_server/)** - Run as REST API server

## Quick Start

```bash
# Test connection to SAP system
abaper connect

# Get source code for an ABAP program
abaper get program ZTEST

# Search for ABAP objects
abaper search objects "Z*"

# Run as REST server
abaper server --port 8080
```

## Configuration

ABAPER can be configured using environment variables or command line flags:

- `SAP_HOST` / `--adt-host` - SAP system hostname
- `SAP_CLIENT` / `--adt-client` - SAP client number
- `SAP_USERNAME` / `--adt-username` - SAP username
- `SAP_PASSWORD` / `--adt-password` - SAP password

For detailed information about each command, click on the command links above.
