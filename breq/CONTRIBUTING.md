# Contributing to bluerequests

Thank you for your interest in contributing! This guide explains the development workflow and standards.

## Development Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/bluefunda/bluerequests.git
   cd bluerequests
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Build and test**
   ```bash
   make build
   make test
   ```

## Validation Sequence

Before submitting a PR, run:

```bash
make fmt      # Format code
make vet      # Static analysis
make test     # Tests with race detector
make build    # Verify build succeeds
```

## Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add bulk stage command for change requests
fix: correct event subscription reconnect logic
docs: update change request lifecycle documentation
refactor: extract gRPC retry logic
test: add TUI key binding tests
chore: bump grpc to 1.81.1
```

The PR title must follow this convention — it is enforced by CI.

## Pull Request Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make your changes following the code patterns in [AGENTS.md](AGENTS.md)
4. Run the validation sequence above
5. Submit a PR with a conventional commit title
6. Fill out the PR template completely

## Code Style

- Run `gofmt` before committing (`make fmt`)
- No unnecessary comments — code should be self-documenting
- Error messages use lowercase, no trailing punctuation
- All exported symbols must have GoDoc comments

## Reporting Issues

Open a [GitHub Issue](https://github.com/bluefunda/bluerequests/issues) with:
- A clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Version (`breq version`) and OS/arch

## License

By contributing, you agree your contributions are licensed under Apache 2.0.
