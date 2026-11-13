-include .env

.PHONY: start dev-db dev-db-down

start:
	cd server/cmd && go run .

dev-db:
	docker compose --env-file server/.env.dev -f docker-compose.dev.yaml up -d

dev-db-down:
	docker compose --env-file server/.env.dev -f docker-compose.dev.yaml down

CERTS_DIR := nginx/certs
KEY_FILE := $(CERTS_DIR)/homeserver.lan.key
CRT_FILE := $(CERTS_DIR)/homeserver.lan.crt

.PHONY: clean certs renew-certs ca-backup ca-restore test

TEST_FLAGS := -v -race -cover -count=1

test:
	cd server && go test $(TEST_FLAGS) ./...

# clean removes the server certificate; the home CA (ca-data volume) stays.
clean:
	@echo "Cleaning up generated files..."
	@rm -f $(KEY_FILE) $(CRT_FILE)
	@echo "Cleaned."

# TLS certificates are made in the certs container (see certs/issue-certs.sh)
# from LAN_DOMAIN and LAN_IP in .env. `docker compose up` runs it too, so
# these targets are for doing it by hand.

# certs issues the certificate if it is missing, expiring or out of date.
certs:
	docker compose run --rm --build certs

# renew-certs issues a new certificate now and reloads nginx.
renew-certs:
	docker compose run --rm --build -e FORCE=1 certs
	docker compose restart nginx

# ca-backup saves the home CA (keep it private: it can sign any site).
ca-backup:
	docker compose build -q certs
	docker compose run --rm -T --entrypoint tar certs czf - -C /ca . > ca-backup.tgz
	@chmod 600 ca-backup.tgz
	@echo "Saved the home CA to ca-backup.tgz; keep it somewhere safe and private."

# ca-restore puts a ca-backup.tgz back (e.g. on a new server), so devices
# that trust the old CA keep working.
ca-restore:
	docker compose build -q certs
	docker compose run --rm -T --entrypoint tar certs xzf - -C /ca < ca-backup.tgz
	@echo "Restored the home CA; run make certs to issue a certificate from it."

.PHONY: dev-build dev-start dev-stop dev-down

dev-build:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml up -d --build 

dev-start:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml start

dev-stop:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml stop

dev-down:
	docker compose --env-file ./server/.env.dev -f docker-compose.dev.yaml down

# --- Auto-deploy: pull main and rebuild once CI passes (see deploy/) ---
.PHONY: autodeploy autodeploy-stop autodeploy-logs

AUTODEPLOY_ENV := AUTODEPLOY_DIR=$(CURDIR) \
	AUTODEPLOY_UID=$$(id -u) AUTODEPLOY_GID=$$(id -g) \
	DOCKER_GID=$$(stat -c %g /var/run/docker.sock)

autodeploy:
	$(AUTODEPLOY_ENV) docker compose --profile autodeploy up -d --build deployer

autodeploy-stop:
	$(AUTODEPLOY_ENV) docker compose --profile autodeploy rm -sf deployer

autodeploy-logs:
	docker logs -f deployer
