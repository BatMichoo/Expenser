# --- STAGE 1: Builder ---
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/web-app ./cmd

# --- STAGE 2: Runner ---
FROM alpine:3.20

# pdftotext reads the text of PDF receipts (sent to Gemini as text: a long
# receipt on one tall PDF page is unreadable as a picture).
RUN apk add --no-cache poppler-utils

WORKDIR /app
COPY --from=builder /app/bin/web-app .

RUN chmod +x web-app

COPY internal/i18n/ internal/i18n/
COPY internal/templates/ internal/templates/
COPY internal/db/ internal/db/
COPY static/ static/

# Run as an unprivileged user: the app only reads its files, and pdftotext
# parses untrusted PDFs.
RUN addgroup -S app && adduser -S -G app -H app
USER app

EXPOSE 8080

CMD ["./web-app"]
