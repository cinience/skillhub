.PHONY: build deps dev setup clean help quickstart test frontend dev-frontend dev-backend

# Build frontend (React + Vite)
frontend:
	cd web && npm install && npm run build

# Build the skillhub binary (includes embedded frontend)
build: frontend
	go build -o skillhub ./cmd/skillhub/

# Build Go only (skip frontend, assumes web/static/ exists)
build-go:
	go build -o skillhub ./cmd/skillhub/

# Download Go dependencies
deps:
	go mod download

# One-click: build + create admin + start server (no Docker needed with SQLite)
# Usage: make quickstart ADMIN_USER=me ADMIN_PASSWORD=pass
quickstart: build
	@echo "Starting SkillHub (SQLite mode, no external dependencies)..."
	SKILLHUB_ADMIN_USER=$(ADMIN_USER) SKILLHUB_ADMIN_PASSWORD=$(ADMIN_PASSWORD) ./skillhub serve

# Start PostgreSQL via Docker (only needed for postgres mode)
pg-up:
	docker run -d --name skillhub-pg \
		-p 5432:5432 \
		-e POSTGRES_USER=skillhub \
		-e POSTGRES_PASSWORD=skillhub \
		-e POSTGRES_DB=skillhub \
		postgres:17-alpine 2>/dev/null || true
	@echo "Waiting for PostgreSQL..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		docker exec skillhub-pg pg_isready -U skillhub >/dev/null 2>&1 && break; \
		sleep 1; \
	done
	@echo "PostgreSQL ready."

# Stop PostgreSQL
pg-down:
	docker rm -f skillhub-pg 2>/dev/null || true

# Full setup: build + create admin user
# Usage: make setup ADMIN_USER=myname ADMIN_PASSWORD=mypass
ADMIN_USER ?= admin
ADMIN_PASSWORD ?= admin123
setup: build
	./skillhub admin create-user --handle $(ADMIN_USER) --role admin --password $(ADMIN_PASSWORD)
	@echo ""
	@echo "Setup complete!"
	@echo "  Start server:  make dev"
	@echo "  Web login:     http://localhost:10070/login"
	@echo "  Username:      $(ADMIN_USER)"

# Start server in development mode
dev: build
	./skillhub serve

# Start frontend dev server (hot reload, proxies API to Go backend)
dev-frontend:
	cd web && npm run dev

# Start Go backend only (for development with frontend dev server)
dev-backend: build-go
	./skillhub serve

# Start everything via docker-compose
docker-up:
	docker compose -f deployments/docker/docker-compose.yml up --build -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# Remove all data and rebuild
clean:
	docker rm -f skillhub-pg 2>/dev/null || true
	rm -f skillhub
	rm -rf data/
	rm -rf web/static/
	rm -rf ~/.skillhub/
	@echo "Cleaned."

# Run tests
test:
	go test ./...

# Show help
help:
	@echo "SkillHub Development"
	@echo ""
	@echo "Quick start (SQLite, no Docker needed):"
	@echo "  make quickstart ADMIN_USER=me ADMIN_PASSWORD=pass"
	@echo ""
	@echo "Or step by step:"
	@echo "  make setup ADMIN_USER=me ADMIN_PASSWORD=pass    # Build + create admin"
	@echo "  make dev                               # Start server on :10070"
	@echo ""
	@echo "Frontend development (hot reload):"
	@echo "  make dev-backend                       # Start Go backend"
	@echo "  make dev-frontend                      # Start Vite dev server (in another terminal)"
	@echo ""
	@echo "PostgreSQL mode:"
	@echo "  make pg-up                             # Start PostgreSQL via Docker"
	@echo "  SKILLHUB_DB_DRIVER=postgres SKILLHUB_DATABASE_URL='postgres://skillhub:skillhub@localhost:5432/skillhub?sslmode=disable' make dev"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up                         # Start everything via docker-compose"
	@echo "  make docker-down                       # Stop docker-compose services"
	@echo ""
	@echo "Targets:"
	@echo "  quickstart    - One-click: build + create admin + start server (SQLite)"
	@echo "  build         - Build frontend + Go binary"
	@echo "  build-go      - Build Go binary only (skip frontend)"
	@echo "  frontend      - Build frontend only"
	@echo "  deps          - Download Go dependencies"
	@echo "  pg-up         - Start PostgreSQL via Docker (for postgres mode)"
	@echo "  pg-down       - Stop PostgreSQL"
	@echo "  setup         - Build + create admin user (ADMIN_USER, ADMIN_PASSWORD)"
	@echo "  dev           - Build all + start server"
	@echo "  dev-frontend  - Start Vite dev server (hot reload)"
	@echo "  dev-backend   - Build Go + start server"
	@echo "  test          - Run tests"
	@echo "  docker-up     - Start everything via docker-compose"
	@echo "  docker-down   - Stop docker-compose services"
	@echo "  clean         - Remove all data, binaries, and build artifacts"
	@echo "  help          - Show this help"
