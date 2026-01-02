.PHONY: help dev build docker clean install-tools

help:
	@echo "Available targets:"
	@echo "  make dev          - Start frontend and backend in development mode"
	@echo "  make build        - Build frontend and backend binaries"
	@echo "  make docker       - Build Docker image"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make install-tools - Install required build tools"

install-tools:
	@command -v node >/dev/null 2>&1 || { echo "Node.js is required"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "Go 1.21+ is required"; exit 1; }
	@cd frontend && npm install

dev:
	@echo "Starting development server..."
	@cd frontend && npm run dev &
	@cd backend && PORT=8080 go run main.go

build:
	@echo "Building frontend..."
	@cd frontend && npm install && npm run build
	@echo "Building backend binary with embedded frontend..."
	@mkdir -p backend/static
	@cp -r frontend/dist/* backend/static/
	@cd backend && CGO_ENABLED=0 GOOS=linux go build -o portal main.go
	@echo "Build complete: backend/portal"

docker:
	@echo "Building Docker image..."
	@docker build -t registry.redval.ovh/server/portal:0.0.8 .
	@echo "Image built successfully"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf frontend/dist frontend/node_modules backend/portal backend/static
	@echo "Clean complete"