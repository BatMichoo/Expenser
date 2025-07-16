include .env

.PHONY: start

start:
	cd server/cmd && go run main.go

CERTS_DIR := nginx/certs
KEY_FILE := $(CERTS_DIR)/homeserver.lan.key
CRT_FILE := $(CERTS_DIR)/homeserver.lan.crt
OPENSSL_CNF := $(CERTS_DIR)/openssl.cnf

.PHONY: clean create-ssl certs test

TEST_FLAGS := -v -race -cover -count=1

test:
	cd server && go test $(TEST_FLAGS) ./...

clean:
	@echo "Cleaning up generated files..."
	@rm -f $(OPENSSL_CNF) $(KEY_FILE) $(CRT_FILE)
	@echo "Cleaned."

create-ssl:
	@printf "[ req ]\n" > "$(OPENSSL_CNF)"
	@printf "default_bits = 2048\n" >> "$(OPENSSL_CNF)"
	@printf "default_keyfile = privkey.pem\n" >> "$(OPENSSL_CNF)"
	@printf "distinguished_name = req_distinguished_name\n" >> "$(OPENSSL_CNF)"
	@printf "req_extensions = req_ext\n" >> "$(OPENSSL_CNF)"
	@printf "x509_extensions = v3_req\n\n" >> "$(OPENSSL_CNF)"
	@printf "[ req_distinguished_name ]\n" >> "$(OPENSSL_CNF)"
	@printf "countryName_default = $(COUNTRY_ISO)\n" >> "$(OPENSSL_CNF)"
	@printf "stateOrProvinceName_default = $(PROVINCE)\n" >> "$(OPENSSL_CNF)"
	@printf "localityName_default = $(CITY)\n" >> "$(OPENSSL_CNF)"
	@printf "organizationName_default = $(ORG_NAME)\n" >> "$(OPENSSL_CNF)"
	@printf "commonName_default = $(LAN_DOMAIN)\n\n" >> "$(OPENSSL_CNF)"
	@printf "[ req_ext ]\n" >> "$(OPENSSL_CNF)"
	@printf "subjectAltName = @alt_names\n\n" >> "$(OPENSSL_CNF)"
	@printf "[ v3_req ]\n" >> "$(OPENSSL_CNF)"
	@printf "subjectAltName = @alt_names\n\n" >> "$(OPENSSL_CNF)"
	@printf "[ alt_names ]\n" >> "$(OPENSSL_CNF)"
	@printf "DNS.1 = $(LAN_DOMAIN)\n" >> "$(OPENSSL_CNF)"
	@printf "DNS.2 = localhost\n" >> "$(OPENSSL_CNF)"
	@printf "IP.1 = 127.0.0.1\n" >> "$(OPENSSL_CNF)"
	@printf "IP.2 = $(LAN_IP)\n" >> "$(OPENSSL_CNF)"

certs: create-ssl $(KEY_FILE) $(CRT_FILE)

$(KEY_FILE) $(CRT_FILE):
	@echo "Generating self-signed certificate for $(LAN_DOMAIN) and $(LAN_IP)..."
	@mkdir -p $(CERTS_DIR)

	openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
	    -keyout "$(KEY_FILE)" -out "$(CRT_FILE)" \
	    -subj "/C=$(COUNTRY_ISO)/ST=$(PROVINCE)/L=$(CITY)/O=$(ORG_NAME)/CN=$(LAN_DOMAIN)" \
	    -extensions v3_req \
	    -config "$(OPENSSL_CNF)"
