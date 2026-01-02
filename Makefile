
.PHONY: dev build docker clean

dev:
	@cd frontend && npm install && npm run dev &
	@cd backend && go run main.go

build:
	@cd frontend && npm install && npm run build
	@cd backend && go build -o dashboard main.go

docker:
	@docker build -t k8s-dashboard:latest .

clean:
	@rm -rf frontend/dist frontend/node_modules backend/dashboard