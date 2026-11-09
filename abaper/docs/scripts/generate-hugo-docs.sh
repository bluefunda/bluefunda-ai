#!/bin/bash

# Script to generate Hugo-compatible documentation
# This script generates markdown files with proper Hugo front matter

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
HUGO_CONTENT_DIR="${PROJECT_ROOT}/docs/hugo/content"
DOCS_DIR="${PROJECT_ROOT}/docs"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to add Hugo front matter to markdown files
add_hugo_frontmatter() {
    local file="$1"
    local title="$2"
    local weight="$3"
    local description="$4"
    
    # Create temporary file with front matter
    cat > "${file}.tmp" << EOF
---
title: "${title}"
description: "${description}"
weight: ${weight}
date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
lastmod: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
draft: false
toc: true
---

EOF
    
    # Add original content (skip first line if it's a title)
    if head -1 "$file" | grep -q "^## "; then
        tail -n +2 "$file" >> "${file}.tmp"
    else
        cat "$file" >> "${file}.tmp"
    fi
    
    # Replace original file
    mv "${file}.tmp" "$file"
}

# Function to process markdown files for Hugo
process_markdown_for_hugo() {
    local source_dir="$1"
    local dest_dir="$2"
    
    log_info "Processing markdown files for Hugo..."
    
    # Create destination directory
    mkdir -p "$dest_dir"
    
    # Copy and process each markdown file
    local weight=10
    for file in "$source_dir"/*.md; do
        if [[ -f "$file" ]]; then
            local filename=$(basename "$file")
            local title=$(echo "$filename" | sed 's/abaper_//' | sed 's/.md$//' | sed 's/_/ /g' | sed 's/\b\w/\u&/g')
            local dest_file="$dest_dir/$filename"
            
            # Copy file
            cp "$file" "$dest_file"
            
            # Determine description based on filename
            local description="Command reference for abaper CLI tool"
            case "$filename" in
                "abaper.md")
                    description="Main abaper CLI tool documentation"
                    title="ABAPER CLI Tool"
                    ;;
                "abaper_get.md")
                    description="Retrieve ABAP object source code"
                    ;;
                "abaper_search.md")
                    description="Search for ABAP objects"
                    ;;
                "abaper_list.md")
                    description="List objects of specified type"
                    ;;
                "abaper_connect.md")
                    description="Test ADT connection to SAP system"
                    ;;
                "abaper_server.md")
                    description="Run abaper as REST API server"
                    ;;
            esac
            
            # Add Hugo front matter
            add_hugo_frontmatter "$dest_file" "$title" "$weight" "$description"
            
            weight=$((weight + 10))
        fi
    done
}

# Function to create Hugo section index
create_hugo_index() {
    local dest_dir="$1"
    
    cat > "$dest_dir/_index.md" << EOF
---
title: "CLI Reference"
description: "Complete command line interface reference for abaper"
weight: 100
date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
lastmod: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
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

\`\`\`bash
# Test connection to SAP system
abaper connect

# Get source code for an ABAP program
abaper get program ZTEST

# Search for ABAP objects
abaper search objects "Z*"

# Run as REST server
abaper server --port 8080
\`\`\`

## Configuration

ABAPER can be configured using environment variables or command line flags:

- \`SAP_HOST\` / \`--adt-host\` - SAP system hostname
- \`SAP_CLIENT\` / \`--adt-client\` - SAP client number
- \`SAP_USERNAME\` / \`--adt-username\` - SAP username
- \`SAP_PASSWORD\` / \`--adt-password\` - SAP password

For detailed information about each command, click on the command links above.
EOF
}

# Main function
main() {
    log_info "Starting Hugo documentation generation for ABAPER..."
    
    # Check if we're in the right directory
    if [[ ! -f "$PROJECT_ROOT/main.go" ]]; then
        log_error "This script must be run from the abaper project directory"
        exit 1
    fi
    
    # Build documentation generator
    log_info "Building documentation generator..."
    cd "$PROJECT_ROOT"
    go build -o cmd/gendocs cmd/gendocs.go
    
    # Generate markdown documentation
    log_info "Generating base markdown documentation..."
    ./cmd/gendocs markdown
    
    # Create Hugo content directory structure
    log_info "Creating Hugo content directory structure..."
    mkdir -p "$HUGO_CONTENT_DIR/cli"
    
    # Process markdown files for Hugo
    process_markdown_for_hugo "$DOCS_DIR/markdown" "$HUGO_CONTENT_DIR/cli"
    
    # Create section index
    create_hugo_index "$HUGO_CONTENT_DIR/cli"
    
    # Create main documentation index if it doesn't exist
    if [[ ! -f "$HUGO_CONTENT_DIR/_index.md" ]]; then
        log_info "Creating main documentation index..."
        cat > "$HUGO_CONTENT_DIR/_index.md" << EOF
---
title: "ABAPER Documentation"
description: "Documentation for the ABAPER ABAP Development Tool"
date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
lastmod: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
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

\`\`\`bash
# Download the latest release
# Or build from source:
git clone https://github.com/bluefunda/abaper.git
cd abaper
go build -o abaper .
\`\`\`

## Quick Start

\`\`\`bash
# Set environment variables
export SAP_HOST="your-sap-host:port"
export SAP_CLIENT="100"
export SAP_USERNAME="your-username"
export SAP_PASSWORD="your-password"

# Test connection
./abaper connect

# Get source code
./abaper get program ZTEST
\`\`\`
EOF
    fi
    
    log_success "Hugo documentation generated successfully!"
    log_info "Hugo content directory: $HUGO_CONTENT_DIR"
    log_info "Base documentation: $DOCS_DIR"
    
    # Show summary
    echo
    log_info "Generated files:"
    find "$HUGO_CONTENT_DIR" -name "*.md" | sort | while read file; do
        echo "  - $file"
    done
    
    echo
    log_info "To use with Hugo:"
    echo "  1. Copy the docs/hugo/content directory to your Hugo site"
    echo "  2. Or set up your Hugo site to use $HUGO_CONTENT_DIR as content directory"
    echo "  3. Run 'hugo server' to preview the documentation"
    
    echo
    log_info "To regenerate documentation, run: $0"
}

# Run main function
main "$@"
