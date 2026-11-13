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
OPENSSL_CNF := $(CERTS_DIR)/openssl.cnf

.PHONY: clean certs renew-certs test

TEST_FLAGS := -v -race -cover -count=1

test:
	cd server && go test $(TEST_FLAGS) ./...

clean:
	@echo "Cleaning up generated files..."
	@rm -f $(OPENSSL_CNF) $(KEY_FILE) $(CRT_FILE)
	@echo "Cleaned."

# certs makes a self-signed certificate for nginx from the values in .env,
# unless one exists already. It lasts CERT_DAYS; renew-certs replaces it.
CERT_DAYS := 365

certs: $(CRT_FILE)

# Sub-makes, not prerequisites: under -j, clean and certs would run at once.
renew-certs:
	@$(MAKE) --no-print-directory clean
	@$(MAKE) --no-print-directory certs

# One recipe writes the OpenSSL config and then the key and certificate
# (a grouped target), so parallel make (-j) can't run openssl twice or
# before its config exists. The extensions make it a server certificate:
# Apple devices refuse TLS certificates without extendedKeyUsage serverAuth.
$(KEY_FILE) $(CRT_FILE) &:
	@test -n "$(LAN_DOMAIN)" || { echo "LAN_DOMAIN is not set (see .env.template)"; exit 1; }
	@test -n "$(LAN_IP)" || { echo "LAN_IP is not set (see .env.template)"; exit 1; }
	@echo "Generating self-signed certificate for $(LAN_DOMAIN) and $(LAN_IP)..."
	@mkdir -p $(CERTS_DIR)
	@printf '%s\n' \
		'[ req ]' \
		'distinguished_name = req_distinguished_name' \
		'x509_extensions = v3_req' \
		'' \
		'[ req_distinguished_name ]' \
		'' \
		'[ v3_req ]' \
		'basicConstraints = critical, CA:FALSE' \
		'keyUsage = critical, digitalSignature, keyEncipherment' \
		'extendedKeyUsage = serverAuth' \
		'subjectAltName = @alt_names' \
		'' \
		'[ alt_names ]' \
		'DNS.1 = $(LAN_DOMAIN)' \
		'DNS.2 = localhost' \
		'IP.1 = 127.0.0.1' \
		'IP.2 = $(LAN_IP)' > "$(OPENSSL_CNF)"
	openssl req -x509 -noenc -days $(CERT_DAYS) -newkey rsa:2048 \
	    -keyout "$(KEY_FILE)" -out "$(CRT_FILE)" \
	    -subj "/C=$(COUNTRY_ISO)/ST=$(PROVINCE)/L=$(CITY)/O=$(ORG_NAME)/CN=$(LAN_DOMAIN)" \
	    -config "$(OPENSSL_CNF)"

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
