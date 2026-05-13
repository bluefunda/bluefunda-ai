# CAI CLI

Command line interface for [BlueFunda AI](https://bluefunda.com/bluefunda-ai/) — context-aware AI agents for SAP operations.

The binary is `ai`. Use it to chat with BlueFunda AI, manage MCP tool integrations, and inspect your account and usage from the terminal.

> **Prefer the browser?** You can also use BlueFunda AI directly at [bluefunda.com/login](https://bluefunda.com/login) — no install required.

## Installation

### One-line installer (macOS and Linux)

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/bluefunda/cai-cli/main/install.sh)"
```

Installs to `/usr/local/bin` if writable, otherwise `~/.local/bin`. Override with `AI_INSTALL_DIR`.

### Homebrew (macOS)

```bash
brew tap bluefunda/tap
brew install --cask ai
```

### From GitHub Releases

Download the binary for your platform from the [releases page](https://github.com/bluefunda/cai-cli/releases/latest).

| Platform       | Archive                              |
|----------------|--------------------------------------|
| macOS (ARM64)  | `ai_<version>_darwin_arm64.zip`      |
| macOS (AMD64)  | `ai_<version>_darwin_amd64.zip`      |
| Linux (AMD64)  | `ai_<version>_linux_amd64.tar.gz`    |
| Linux (ARM64)  | `ai_<version>_linux_arm64.tar.gz`    |

Linux users can also install via `.deb` or `.rpm`:

**Debian / Ubuntu:**
```bash
VER=$(curl -fsSL https://api.github.com/repos/bluefunda/cai-cli/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -sL "https://github.com/bluefunda/cai-cli/releases/download/v${VER}/ai_${VER}_linux_${ARCH}.deb" -o ai.deb
sudo dpkg -i ai.deb
```

**RHEL / Fedora / Rocky:**
```bash
VER=$(curl -fsSL https://api.github.com/repos/bluefunda/cai-cli/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
sudo dnf install "https://github.com/bluefunda/cai-cli/releases/download/v${VER}/ai_${VER}_linux_${ARCH}.rpm"
```

### From Source

```bash
go install github.com/bluefunda/cai-cli/cmd/ai@latest
```

### Verify checksums

```bash
sha256sum -c checksums.txt
```

## Quick Start

```bash
# Authenticate (opens browser for OAuth device flow)
ai login

# Verify connection
ai health

# Start a chat
ai chat start "Summarize last quarter's sales from SAP"

# List available models
ai model list
```

## Commands

| Command | Description |
|---------|-------------|
| `ai login` | Authenticate via OAuth2 device flow (Keycloak) |
| `ai health` | Check gRPC connection to the backend |
| `ai chat` | Manage chat sessions: `list`, `start`, `history`, `context`, `title`, `stop` |
| `ai model` | List available LLM models |
| `ai mcp` | Manage MCP tool integrations |
| `ai user` | Show account information |
| `ai billing` | Show billing and usage |
| `ai ratelimit` | Show current rate-limit status |
| `ai version` | Print CLI version |

Run `ai <command> --help` for full options.

## Authentication

CAI CLI uses the **OAuth2 device authorization flow**:

1. `ai login` requests a device code
2. Your browser opens the verification URL
3. The CLI polls for authorization completion
4. Access and refresh tokens are stored locally in `~/.cai/`

Tokens are refreshed automatically — you only need to log in once.

## Configuration

Configuration is loaded from `~/.cai/config.yaml`. Example:

```yaml
# ~/.cai/config.yaml
endpoint: grpc.bluefunda.com:443
```

## About BlueFunda AI

BlueFunda AI brings intelligent automation to SAP — context-aware, no-code agents that execute complex tasks in minutes. Capabilities include:

- **AI-Powered SAP Management** — automate workflows across operations, data, and analytics
- **No-Code Service Generation** — create SAP OData services from plain-text prompts
- **Conversational Analytics** — ask data-driven questions in English, get visual insights from SAP
- **Cross-Cloud Orchestration** — manage S/4HANA on AWS, Azure, and hybrid setups

BlueFunda AI also powers the AI features in [ABAPer](https://abaper.bluefunda.com).

Learn more at [bluefunda.com/bluefunda-ai](https://bluefunda.com/bluefunda-ai/).

## License

See [LICENSE](./LICENSE).
