# Documentation Generation

This section covers how to generate and maintain documentation for the ABAPER CLI tool.

## Available Documentation Formats

ABAPER supports generating documentation in multiple formats:

- **Man pages** - Traditional Unix manual pages
- **Markdown** - For GitHub, wikis, and static site generators
- **YAML** - Structured data format
- **reStructuredText** - For Sphinx and Read the Docs
- **Hugo-ready Markdown** - With front matter for Hugo static sites

## Quick Start

### Generate All Documentation

```bash
# Using the built-in command (if integrated)
./abaper docs

# Using the standalone generator
go run cmd/gendocs.go all

# Using the Makefile
make docs
```

### Generate Specific Formats

```bash
# Man pages only
make docs-man

# Markdown only  
make docs-markdown

# Hugo-compatible documentation
./docs/scripts/generate-hugo-docs.sh
```

## Manual Generation Steps

### 1. Install Dependencies

```bash
# Add documentation dependencies to go.mod
go get github.com/spf13/cobra/doc@latest
```

### 2. Build Documentation Generator

```bash
go build -o cmd/gendocs cmd/gendocs.go
```

### 3. Generate Documentation

```bash
# Generate all formats
./cmd/gendocs all

# Or generate specific formats
./cmd/gendocs man
./cmd/gendocs markdown
./cmd/gendocs yaml
./cmd/gendocs rest
```

### 4. Generate Hugo Documentation

```bash
# Make script executable
chmod +x docs/scripts/generate-hugo-docs.sh

# Generate Hugo-compatible docs
./docs/scripts/generate-hugo-docs.sh
```

## Output Structure

```
docs/
├── man/                 # Man pages (.1 files)
│   ├── abaper.1
│   ├── abaper_get.1
│   ├── abaper_search.1
│   └── ...
├── markdown/           # Standard markdown files
│   ├── abaper.md
│   ├── abaper_get.md
│   └── ...
├── yaml/              # YAML format documentation
└── rest/              # reStructuredText files

hugo/
└── content/           # Hugo-ready content with front matter
    ├── _index.md
    └── cli/
        ├── _index.md
        ├── abaper.md
        └── ...
```

## Using Generated Documentation

### Man Pages

```bash
# Install man pages system-wide (requires sudo)
make install-man

# View man pages
man abaper
man abaper-get

# Uninstall man pages
make uninstall-man
```

### Hugo Website

```bash
# Copy to your Hugo site
cp -r docs/hugo/content/* /path/to/your/hugo/site/content/

# Or create a new Hugo site
hugo new site abaper-docs
cp -r docs/hugo/content/* abaper-docs/content/
cd abaper-docs
hugo server
```

### GitHub Pages

The documentation is automatically deployed to GitHub Pages via GitHub Actions when changes are pushed to the main branch.

## Automation

### GitHub Actions

Documentation is automatically generated on:
- Push to main/develop branches
- Pull requests 
- New releases

The workflow:
1. Builds the documentation generator
2. Generates all documentation formats
3. Creates Hugo-compatible files
4. Validates man pages
5. Deploys to GitHub Pages (main branch only)

### Local Development

```bash
# Watch for changes and regenerate docs
# (Add this to your development workflow)
find . -name "*.go" | entr -r make docs
```

## Customization

### Modifying the Generator

Edit `cmd/gendocs.go` to:
- Add new output formats
- Customize man page headers
- Adjust file naming conventions
- Add custom processing

### Hugo Front Matter

The Hugo script automatically adds appropriate front matter:

```yaml
---
title: "Command Name"
description: "Command description"
weight: 10
date: "2025-07-31T12:00:00Z"
lastmod: "2025-07-31T12:00:00Z"
draft: false
toc: true
---
```

### Custom Styling

For Hugo sites, add custom CSS/templates to style the CLI documentation appropriately.

## Troubleshooting

### Common Issues

1. **Missing cobra/doc dependency**
   ```bash
   go get github.com/spf13/cobra/doc@latest
   go mod tidy
   ```

2. **Permission denied for scripts**
   ```bash
   chmod +x docs/scripts/generate-hugo-docs.sh
   ```

3. **Man page validation errors**
   ```bash
   # Check man page syntax
   man --warnings -l docs/man/abaper.1
   ```

4. **Hugo front matter issues**
   - Ensure YAML front matter is properly formatted
   - Check for special characters in titles/descriptions

### Debug Mode

```bash
# Enable verbose output
./docs/scripts/generate-hugo-docs.sh --verbose

# Check generated file structure
find docs -type f | sort
```

## Integration with CI/CD

The documentation generation integrates with:

- **GitHub Actions** - Automatic generation and deployment
- **Pre-commit hooks** - Generate docs before commits
- **Release process** - Update docs with new versions
- **Hugo deployments** - Direct integration with static sites

## Best Practices

1. **Keep command descriptions current** - Update long descriptions when functionality changes
2. **Use consistent formatting** - Follow established patterns for examples and usage
3. **Test man pages** - Validate syntax and rendering
4. **Version documentation** - Include version info in generated docs
5. **Automate updates** - Use CI/CD to keep docs in sync with code changes
