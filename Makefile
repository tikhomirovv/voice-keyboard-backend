# Development commands
up:
	docker compose -f docker-compose.yml up -d
stop:
	docker compose -f docker-compose.yml stop
down:
	docker compose -f docker-compose.yml down

# Production commands
up-prod:
	docker compose -f docker-compose.prod.yml up -d
stop-prod:
	docker compose -f docker-compose.prod.yml stop
down-prod:
	docker compose -f docker-compose.prod.yml down
logs-prod:
	docker compose -f docker-compose.prod.yml logs -f

# Build commands
build:
	docker build -t voice-key-backend-go .
build-prod:
	docker build -t voice-key-backend-go:prod .

# Database commands
migrate:
	go run main.go database:migrations:migrate
rollback:
	go run main.go database:migrations:rollback
migrate-prod:
	docker compose -f docker-compose.prod.yml exec app /app/voice-key database:migrations:migrate
rollback-prod:
	docker compose -f docker-compose.prod.yml exec app /app/voice-key database:migrations:rollback

# Development commands
wire:
	wire gen gitlab.com/voice-keyboard/backend-go/bootstrap
run:
	go run main.go app:start
tests:
	go test ./...

# Utility commands
clean:
	docker system prune -f
	docker volume prune -f
clean-prod:
	docker compose -f docker-compose.prod.yml down -v
	docker system prune -f

# Health checks
health:
	curl -f http://localhost:8080/health || exit 1
health-prod:
	curl -f https://localhost/health || exit 1
