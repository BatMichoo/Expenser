# Shortcuts for developing Expenser. In production it runs inside the home
# server (github.com/BatMichoo/homeserver), which builds server/Dockerfile.

.PHONY: start dev-db dev-db-down test dev-build dev-start dev-stop dev-down

start:
	cd server/cmd && go run .

dev-db:
	docker compose --env-file server/.env.dev -f docker-compose.dev.yaml up -d

dev-db-down:
	docker compose --env-file server/.env.dev -f docker-compose.dev.yaml down

TEST_FLAGS := -v -race -cover -count=1

test:
	cd server && go test $(TEST_FLAGS) ./...

dev-build:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml up -d --build

dev-start:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml start

dev-stop:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml stop

dev-down:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml down
