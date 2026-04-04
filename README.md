# SkillHub

Self-hosted agent skill registry. Publish, version, and distribute agent skills across your organization — with a Web UI, REST API, and CLI client built on the [ClawHub](https://github.com/openclaw/clawhub) protocol. Ideal for enterprises building their own internal agent skill registry.

[English](README.md) | [中文](README_CN.md)

## Why SkillHub?

- **Own your data** — Run on your infrastructure. No vendor lock-in, no external dependencies.
- **Single binary** — One Go binary serves the registry, Web UI, and CLI. Deploy anywhere.
- **Git-native versioning** — Every skill version is a Git commit in a bare repository. Full history, diffs, and rollbacks built in.
- **Instant search** — Meilisearch-powered full-text search with typo tolerance across skill names, summaries, and tags.
- **ClawHub compatible** — Implements the ClawHub registry protocol. Skills published to SkillHub work with any ClawHub-compatible client.
- **Webhook import** — Push to GitHub/GitLab/Gitea and skills are auto-imported and published.
- **Auth & RBAC** — bcrypt password authentication, scoped API tokens, role-based access control (admin / moderator / user).

## Quick Start

### Docker Compose (Recommended)

```bash
git clone https://github.com/cinience/skillhub.git
cd skillhub
make docker-up
```

This starts PostgreSQL, Meilisearch, and SkillHub in one command. An admin user (`admin` / `admin123`) is created automatically on first boot.

Customize admin credentials:

```bash
SKILLHUB_ADMIN_USER=myname SKILLHUB_ADMIN_PASSWORD=secret make docker-up
```

Open http://localhost:8080.

### Local Development

Prerequisites: Go 1.25+, Docker

```bash
# One command: start deps + build + create admin + start server
make quickstart ADMIN_USER=admin ADMIN_PASSWORD=admin123
```

Or step by step:

```bash
make deps-up                                        # Start PostgreSQL + Meilisearch
make setup ADMIN_USER=admin ADMIN_PASSWORD=admin123  # Build + create admin
make dev                                             # Start server on :8080
```

### Single Binary

```bash
go build -o skillhub ./cmd/skillhub/

SKILLHUB_ADMIN_USER=admin \
SKILLHUB_ADMIN_PASSWORD=admin123 \
./skillhub serve
```

The server auto-runs migrations and creates the admin user on first startup. Subsequent restarts are idempotent.

## CLI

The `skillhub` binary is both the server and the client.

```bash
# Auth
skillhub login                              # Interactive login (registry URL + API token)
skillhub whoami                             # Show current user

# Discover
skillhub search "browser automation"        # Full-text search
skillhub list --sort downloads              # Browse registry
skillhub inspect agent-browser              # Skill details + version history

# Install & manage
skillhub install agent-browser              # Install latest version
skillhub install agent-browser --version 2.0.0
skillhub installed                          # List local skills
skillhub update --all                       # Update all installed skills
skillhub uninstall agent-browser            # Remove

# Publish
skillhub publish ./my-skill \
  --slug my-skill --version 1.0.0 \
  --tags "coding,automation" \
  --summary "A useful coding skill"

# Admin
skillhub admin create-user --handle alice --role admin --password secret
skillhub admin create-token --user alice --label "CI"
skillhub admin set-password --user alice --password newpass
```

Skills are installed to `~/.skillhub/skills/` by default. Customize via `skills_dir` in `~/.skillhub/config.yaml`.

## Architecture

```mermaid
graph LR
    subgraph Clients
        W["🌐 Web UI"]
        C["⌨️ CLI"]
        A["🤖 AI Agents"]
    end

    subgraph SkillHub["SkillHub Server"]
        direction TB
        R["Router & Middleware<br/>Auth · Rate Limit · Logging"]
        S["Service Layer<br/>Skill · Auth · Import"]
        R --> S
    end

    subgraph Storage
        G[("Git<br/>Repos")]
        M[("Meili<br/>Search")]
        P[("Postgres<br/>DB")]
    end

    W & C & A --> R
    S --> G & M & P
```

## Project Structure

```
skillhub/
├── cmd/skillhub/           # Entry point — server + CLI routing
│   └── main.go
├── internal/
│   ├── auth/                # bcrypt passwords, HMAC API tokens, sessions
│   ├── cli/                 # CLI client (config, HTTP client, commands, output)
│   ├── config/              # YAML config + environment variable overrides
│   ├── gitstore/            # Bare Git repo storage, mirror push, webhook import
│   ├── handler/             # HTTP handlers (skill, auth, search, admin, web UI)
│   ├── middleware/          # Auth, rate limiting, request ID, logging
│   ├── model/               # Domain models (User, Skill, Version, Token, Star)
│   ├── repository/          # PostgreSQL repositories (sqlx)
│   ├── search/              # Meilisearch integration
│   ├── server/              # Server bootstrap, routing, auto-setup
│   └── service/             # Business logic (publish, download, versioning)
├── migrations/              # SQL migration files (golang-migrate)
├── configs/                 # Default config (skillhub.yaml)
├── web/templates/           # Server-rendered HTML (Go templates)
├── deployments/docker/      # Dockerfile + docker-compose.yml
├── Makefile
└── go.mod
```

## Configuration

Configuration is loaded from `configs/skillhub.yaml` with environment variable overrides:

| Variable | Description | Default |
|---|---|---|
| `SKILLHUB_PORT` | Server port | `8080` |
| `SKILLHUB_HOST` | Bind address | `0.0.0.0` |
| `SKILLHUB_BASE_URL` | Public URL | `http://localhost:8080` |
| `SKILLHUB_DATABASE_URL` | PostgreSQL connection string | `postgres://skillhub:skillhub@localhost:5432/skillhub?sslmode=disable` |
| `SKILLHUB_MEILI_URL` | Meilisearch URL | `http://localhost:7700` |
| `SKILLHUB_MEILI_KEY` | Meilisearch API key | _(empty)_ |
| `SKILLHUB_GIT_PATH` | Git storage path | `./data/repos` |
| `SKILLHUB_ADMIN_USER` | Auto-create admin on startup | _(empty)_ |
| `SKILLHUB_ADMIN_PASSWORD` | Admin password | _(empty)_ |
| `SKILLHUB_CONFIG` | Config file path | `configs/skillhub.yaml` |

### CLI Configuration

The CLI client stores its config in `~/.skillhub/config.yaml`:

```yaml
registry: http://localhost:8080    # Registry server URL
token: clh_xxxxxxxxxxxx            # API token (set via `skillhub login`)
skills_dir: ~/.skillhub/skills    # Skill install directory (optional)
```

| Field | Description | Default |
|---|---|---|
| `registry` | Registry server URL | `http://localhost:8080` |
| `token` | API token for authentication | _(set via `skillhub login`)_ |
| `skills_dir` | Local skill install directory | `~/.skillhub/skills` |

## API

### Public

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/skills` | List skills |
| `GET` | `/api/v1/skills/:slug` | Get skill details |
| `GET` | `/api/v1/skills/:slug/versions` | List versions |
| `GET` | `/api/v1/skills/:slug/versions/:version` | Get specific version |
| `GET` | `/api/v1/skills/:slug/file` | Get skill file content |
| `GET` | `/api/v1/search?q=...` | Full-text search |
| `GET` | `/api/v1/download?slug=...&version=...` | Download skill ZIP |
| `GET` | `/api/v1/resolve` | Resolve skill version |
| `GET` | `/healthz` | Liveness check |
| `GET` | `/readyz` | Readiness check |
| `GET` | `/.well-known/clawhub.json` | ClawHub protocol discovery |

### Authenticated

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/whoami` | Current user info |
| `POST` | `/api/v1/skills` | Publish a skill |
| `DELETE` | `/api/v1/skills/:slug` | Soft-delete a skill |
| `POST` | `/api/v1/skills/:slug/undelete` | Restore a deleted skill |
| `POST` | `/api/v1/stars/:slug` | Star a skill |
| `DELETE` | `/api/v1/stars/:slug` | Unstar a skill |

### Admin

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/users` | List users |
| `POST` | `/api/v1/users` | Create user |
| `POST` | `/api/v1/tokens` | Create API token |
| `POST` | `/api/v1/users/ban` | Ban/unban user |
| `POST` | `/api/v1/users/role` | Set user role |

### Webhooks

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/webhooks/github` | GitHub push webhook |
| `POST` | `/api/v1/webhooks/gitlab` | GitLab push webhook |
| `POST` | `/api/v1/webhooks/gitea` | Gitea push webhook |

## Tech Stack

| Component | Technology |
|---|---|
| Language | Go 1.25 |
| Web Framework | [Gin](https://github.com/gin-gonic/gin) |
| Database | PostgreSQL 17 + [sqlx](https://github.com/jmoiron/sqlx) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Search | [Meilisearch](https://www.meilisearch.com/) |
| Git Storage | [go-git](https://github.com/go-git/go-git) |
| Auth | bcrypt + HMAC tokens |
| Versioning | [Semantic Versioning](https://semver.org/) via [semver](https://github.com/Masterminds/semver) |

## License

MIT
