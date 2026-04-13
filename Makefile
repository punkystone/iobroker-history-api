docker-build:
	docker compose build

docker-up:
	docker compose up

docker-down:
	docker compose down

lint:
	golangci-lint run 