# Contributing to SkillHub

Thank you for your interest in contributing to SkillHub! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.25+
- Node.js 22+
- Git

### Getting Started

```bash
git clone https://github.com/cinience/skillhub.git
cd skillhub

# Build everything and start with a test admin account
make quickstart ADMIN_USER=admin ADMIN_PASSWORD=admin123
```

### Development Workflow

For active development with hot reload:

```bash
# Terminal 1: Start Go backend
make dev-backend

# Terminal 2: Start Vite dev server (hot reload for frontend)
make dev-frontend
```

The Vite dev server proxies API requests to the Go backend automatically.

### Running Tests

```bash
make test
```

## Project Structure

- `cmd/skillhub/` — Entry point (server + CLI)
- `internal/` — All Go packages (not importable externally)
- `web/` — React frontend (Vite + TypeScript)
- `web/templates/` — Server-rendered HTML templates (fallback)
- `configs/` — Default configuration
- `deployments/` — Docker files

## Making Changes

### Code Style

- **Go**: Follow standard Go conventions. Run `go vet ./...` before submitting.
- **TypeScript/React**: Follow the existing ESLint configuration in `web/`.
- Keep changes focused — one concern per PR.

### Commit Messages

Use clear, descriptive commit messages:

- `fix: resolve token expiration check`
- `feat: add skill version comparison`
- `docs: update API documentation`
- `refactor: simplify auth middleware`

### Pull Request Process

1. Fork the repository and create a feature branch from `main`
2. Make your changes with appropriate tests
3. Ensure `make test` passes
4. Ensure `make build` succeeds
5. Submit a pull request with a clear description of the changes

### What Makes a Good PR

- Focused on a single change
- Includes tests for new functionality
- Updates documentation if needed
- Passes all existing tests
- Has a clear description of what and why

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include steps to reproduce for bugs
- For security vulnerabilities, see [SECURITY.md](SECURITY.md)

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
