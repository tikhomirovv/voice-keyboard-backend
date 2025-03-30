up:
	docker compose -f docker-compose.yml up -d
stop:
	docker compose -f docker-compose.yml stop
wire:
	wire gen gitlab.com/voice-keyboard/backend-go/bootstrap
run:
	go run main.go app:start
migrate:
	go run main.go database:migrations:migrate
rollback:
	go run main.go database:migrations:rollback
tests:
	go test ./...
