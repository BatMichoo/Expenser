# File: ./server/Dockerfile

# --- STAGE 1: Builder ---
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/web-app ./cmd/web

# --- STAGE 2: Runner ---
FROM alpine:3.20

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/bin/web-app .

# Make the copied binary executable (important for previous issue)
RUN chmod +x web-app

# --- FIX TEMPLATE PATH HERE ---
# Copy the entire 'internal/templates' directory (including 'internal/')
# to the current WORKDIR (/app). This makes the path inside the container:
# /app/internal/templates/
COPY internal/templates/ internal/templates/

# Copy static/ to /app/static/ (this was already correct)
COPY static/ static/

EXPOSE 8080

# Command to run the executable
CMD ["/app/web-app"]

