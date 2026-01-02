# Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --production=false
COPY frontend/ ./
RUN npm run build

# Build backend binary with embedded static files
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/main.go ./
# Copy frontend dist files into static directory for embedding
COPY --from=frontend-builder /app/frontend/dist ./static/
# Build with CGO disabled for minimal scratch compatibility
RUN CGO_ENABLED=0 GOOS=linux go build -o portal main.go

# Final minimal image
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /app/portal ./
# Optional: copy config for demo mode
COPY backend/config.yaml ./

EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O- http://localhost:8080/health || exit 1

RUN apk add curl --no-cache
CMD ["./portal"]