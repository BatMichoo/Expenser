# Issues nginx's TLS certificate from a private home certificate authority.
# Runs once per `docker compose up` (nginx waits for it) and exits.
FROM alpine:3.20

RUN apk add --no-cache openssl

COPY issue-certs.sh /usr/local/bin/issue-certs
RUN chmod +x /usr/local/bin/issue-certs

ENTRYPOINT ["issue-certs"]
