# --- STAGE 1: Builder ---
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/web-app ./cmd

# --- STAGE 2: Runner ---
FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/bin/web-app .

RUN chmod +x web-app

COPY internal/templates/ internal/templates/

COPY static/ static/

EXPOSE 8080

CMD ["./web-app"]

