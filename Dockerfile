# File: ./server/Dockerfile

# --- STAGE 1: Builder ---
FROM golang:1.22-alpine AS builder

# Set working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files and download dependencies
# This step is optimized for Docker caching:
# If go.mod/go.sum don't change, this layer is reused.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application
# -ldflags="-s -w" reduces the binary size by stripping debug info
# -o /app/bin/web-app specifies the output path and name for the executable
# ./cmd/web is the path to your main package
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/web-app ./cmd/web

# --- STAGE 2: Runner ---
FROM alpine:3.20.0

# Set working directory
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/bin/web-app .

# Copy static assets and templates
# Adjust these paths based on your actual project structure
COPY internal/templates/ templates/
COPY static/ static/

# Expose the port your Go application listens on (default for Gin/Echo is 8080)
EXPOSE 8080

# Command to run the executable
CMD ["./web-app"]
