---
title: "ABAPER Documentation"
description: "Documentation for the ABAPER ABAP Development Tool"
date: 2025-08-01T03:31:55Z
lastmod: 2025-08-01T03:31:55Z
draft: false
---

# ABAPER Documentation

Welcome to the ABAPER documentation. ABAPER is a comprehensive CLI tool and REST API for interacting with SAP ABAP systems via ADT (ABAP Development Tools).

## Sections

- **[CLI Reference](cli/)** - Complete command line interface reference
- **[API Reference](api/)** - REST API documentation (if available)
- **[Getting Started](getting-started/)** - Quick start guide and tutorials

## Features

- Retrieve ABAP object source code
- Search for ABAP objects by pattern
- List packages and other object types
- Test ADT connectivity
- REST API server mode
- POSIX-compliant command line interface

## Installation

```bash
# Download the latest release
# Or build from source:
git clone https://github.com/bluefunda/abaper.git
cd abaper
go build -o abaper .
```

## Quick Start

```bash
# Set environment variables
export SAP_HOST="your-sap-host:port"
export SAP_CLIENT="100"
export SAP_USERNAME="your-username"
export SAP_PASSWORD="your-password"

# Test connection
./abaper connect

# Get source code
./abaper get program ZTEST
```
