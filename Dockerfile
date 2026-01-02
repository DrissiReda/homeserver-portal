FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

FROM golang:1.21-alpine AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/main.go ./
RUN go build -o dashboard main.go

FROM nginx:alpine
COPY --from=frontend /app/frontend/dist /usr/share/nginx/html
COPY --from=backend /app/dashboard /usr/local/bin/
COPY nginx.conf /etc/nginx/nginx.conf
COPY backend/config.yaml /etc/dashboard/config.yaml

EXPOSE 80
CMD ["/bin/sh", "-c", "/usr/local/bin/dashboard & nginx -g 'daemon off;'"]