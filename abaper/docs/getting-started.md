# Documentation Generation for ABAPER

This guide provides step-by-step instructions for generating comprehensive documentation for your ABAPER CLI tool.

## Summary of Steps

Here's a complete workflow to generate man pages, CLI help, and Hugo-compatible markdown documentation:

### 1. **Install Dependencies**
```bash
cd ~/Downloads/abaper
go get github.com/spf13/cobra/doc@latest
go mod tidy
```

### 2. **Generate All Documentation Formats**
```bash
# Generate everything at once
make docs

# Or generate specific formats
make docs-man        # Man pages
make docs-markdown   # Standard markdown
make docs-hugo       # Hugo-compatible with front matter
make docs-yaml       # YAML format
make docs-rest       # reStructuredText
```

### 3. **Install and Test Man Pages**
```bash
# Install man pages system-wide
sudo make install-man

# Test the man pages
man abaper
man abaper-get
man abaper-search
```

### 4. **Use Hugo Documentation**
```bash
# For existing Hugo site
cp -r docs/hugo/content/* /path/to/your/hugo/site/content/

# For new Hugo site
hugo new site abaper-docs
cp -r docs/hugo/content/* abaper-docs/content/
cd abaper-docs
hugo server
```

## File Structure After Generation

```
abaper/
├── docs/
│   ├── man/                 # Man pages (.1 files)
│   │   ├── abaper.1
│   │   ├── abaper_get.1
│   │   ├── abaper_search.1
│   │   ├── abaper_list.1
│   │   ├── abaper_connect.1
│   │   └── abaper_server.1
│   ├── markdown/           # Standard markdown
│   │   ├── abaper.md
│   │   ├── abaper_get.md
│   │   └── ...
│   ├── yaml/              # YAML documentation
│   └── rest/              # reStructuredText
│   ├── hugo/
│   │   └── content/           # Hugo-ready with front matter
│   │       ├── _index.md      # Main documentation index
│   │       └── cli/
│   │           ├── _index.md  # CLI section index
│   │           ├── abaper.md
│   │           └── ...
│   └── scripts/
│       └── generate-hugo-docs.sh  # Hugo documentation generator
├── cmd/
│   └── gendocs.go         # Documentation generator
├── Makefile               # Build automation
└── docs/doc-generation.md # Detailed documentation guide
```

## Available Make Targets

```bash
# Show all available targets
make help

# Generate specific documentation formats
make docs-man        # Generate man pages only
make docs-markdown   # Generate markdown only
make docs-hugo       # Generate Hugo-compatible docs
make docs-yaml       # Generate YAML format
make docs-rest       # Generate reStructuredText

# Generate everything
make docs-all        # All formats including Hugo
make docs            # Alias for docs-all

# Management
make clean-docs      # Remove all generated documentation
make install-man     # Install man pages system-wide (requires sudo)
make uninstall-man   # Remove installed man pages (requires sudo)
```

## Manual Steps (Alternative)

If you prefer to run commands manually:

### Step 1: Build the Documentation Generator
```bash
go build -o cmd/gendocs cmd/gendocs.go
```

### Step 2: Generate Base Documentation
```bash
# Generate all formats
./cmd/gendocs all

# Or generate specific formats
./cmd/gendocs man
./cmd/gendocs markdown
./cmd/gendocs yaml
./cmd/gendocs rest
```

### Step 3: Generate Hugo Documentation
```bash
chmod +x docs/scripts/generate-hugo-docs.sh
./docs/scripts/generate-hugo-docs.sh
```

## Hugo Integration Examples

### For Existing Hugo Site
```bash
# Copy CLI documentation to existing site
cp -r docs/hugo/content/cli /path/to/your/hugo/site/content/

# Update your Hugo site's navigation to include CLI docs
# Add to your config.yaml or config.toml:
# menu:
#   main:
#     - name: "CLI Reference"
#       url: "/cli/"
#       weight: 100
```

### Create New Hugo Documentation Site
```bash
# Create new Hugo site for documentation
hugo new site abaper-docs
cd abaper-docs

# Install a documentation theme (example with Docsy)
git init
git submodule add https://github.com/google/docsy.git themes/docsy

# Copy generated content
cp -r ../docs/hugo/content/* content/

# Configure Hugo (example config.yaml)
cat > config.yaml << EOF
baseURL: 'https://your-domain.com'
languageCode: 'en-us'
title: 'ABAPER Documentation'
theme: 'docsy'

params:
  github_repo: 'https://github.com/bluefunda/abaper'
  github_branch: 'main'

markup:
  goldmark:
    renderer:
      unsafe: true
  highlight:
    style: github
    lineNos: true

menu:
  main:
    - name: "CLI Reference"
      url: "/cli/"
      weight: 100
EOF

# Start Hugo development server
hugo server
```

## Automation with GitHub Actions

The repository includes a GitHub Actions workflow (`.github/workflows/docs.yml`) that automatically:

1. **Generates documentation** on every push to main/develop
2. **Validates man pages** for syntax correctness
3. **Creates Hugo-compatible files** with proper front matter
4. **Deploys to GitHub Pages** (on main branch)
5. **Uploads artifacts** for manual download

### Triggering Documentation Updates

```bash
# Any change to Go files triggers documentation regeneration
git add .
git commit -m "Update CLI commands"
git push origin main

# Check the Actions tab in GitHub to see the workflow run
# Documentation will be automatically updated
```

## Customization Options

### Modify Man Page Headers

Edit `cmd/gendocs.go` to customize man page metadata:

```go
if err := doc.GenManTree(rootCmd, &doc.GenManHeader{
    Title:   "ABAPER",           // Man page title
    Section: "1",               // Man section (1 = user commands)
    Manual:  "ABAPER Manual",   // Manual name
    Source:  "ABAPER v0.0.1",   // Source attribution
}, manDir); err != nil {
    return fmt.Errorf("failed to generate man pages: %v", err)
}
```

### Customize Hugo Front Matter

Edit `docs/scripts/generate-hugo-docs.sh` to modify the front matter template:

```bash
add_hugo_frontmatter() {
    local file="$1"
    local title="$2"
    local weight="$3"
    local description="$4"
    
    cat > "${file}.tmp" << EOF
---
title: "${title}"
description: "${description}"
weight: ${weight}
date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
lastmod: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
draft: false
toc: true
categories: ["CLI"]
tags: ["command-line", "reference"]
---
EOF
}
```

### Add Custom Styling for Hugo

Create custom CSS for CLI documentation:

```css
/* assets/scss/custom.scss */
.cli-command {
  background: #f8f9fa;
  border-left: 4px solid #007bff;
  padding: 1rem;
  margin: 1rem 0;
}

.cli-flag {
  font-family: 'Courier New', monospace;
  background: #e9ecef;
  padding: 0.2rem 0.4rem;
  border-radius: 3px;
}

.cli-example {
  background: #2d3748;
  color: #e2e8f0;
  padding: 1rem;
  border-radius: 6px;
  overflow-x: auto;
}
```

## Testing and Validation

### Test Man Pages
```bash
# Validate man page syntax
man --warnings -l docs/man/abaper.1

# Test rendering
MANWIDTH=80 man --local-file docs/man/abaper.1

# Check all man pages
for manpage in docs/man/*.1; do
    echo "Checking $manpage"
    man --warnings -l "$manpage" > /dev/null && echo "✓ Valid"
done
```

### Test Hugo Content
```bash
# Check Hugo front matter syntax
hugo --source docs/hugo --destination /tmp/hugo-test

# Validate markdown
for md in docs/hugo/content/cli/*.md; do
    echo "Checking $md"
    head -10 "$md" | grep -q "^---$" && echo "✓ Has front matter"
done
```

### Test CLI Help
```bash
# Verify CLI help works
./abaper --help
./abaper get --help
./abaper search --help

# Compare with generated docs
diff <(./abaper --help) <(head -20 docs/markdown/abaper.md)
```

## Troubleshooting

### Common Issues and Solutions

1. **"cobra/doc not found" error**
   ```bash
   go get github.com/spf13/cobra/doc@latest
   go mod tidy
   ```

2. **Permission denied on scripts**
   ```bash
   chmod +x docs/scripts/generate-hugo-docs.sh
   ```

3. **Man page validation fails**
   ```bash
   # Check for common issues
   man --warnings -l docs/man/abaper.1
   # Look for malformed markup or missing sections
   ```

4. **Hugo front matter issues**
   ```bash
   # Validate YAML syntax
   python -c "import yaml; yaml.safe_load(open('docs/hugo/content/cli/abaper.md').read().split('---')[1])"
   ```

5. **Missing documentation files**
   ```bash
   # Check if all commands are included
   ./abaper --help | grep -E '^  [a-z]+' | wc -l
   ls docs/markdown/abaper_*.md | wc -l
   # Numbers should match
   ```

## Integration with Development Workflow

### Pre-commit Hook
```bash
# Add to .git/hooks/pre-commit
#!/bin/bash
echo "Regenerating documentation..."
make docs-markdown > /dev/null 2>&1
git add docs/markdown/
```

### Release Process
```bash
# Include in your release script
echo "Updating documentation for release..."
make clean-docs
make docs
git add docs/
git commit -m "docs: update for release $VERSION"
```

### Development Watch
```bash
# Auto-regenerate docs when Go files change
find . -name "*.go" | entr -r make docs-markdown
```

This comprehensive setup ensures your ABAPER CLI tool has professional, accessible documentation across multiple formats, with automated generation and deployment capabilities.
