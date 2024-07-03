# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o tusk ./cmd/server/

# Run stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/tusk .

# Copy static files and templates
COPY web /app/web

# Expose the port the app runs on
EXPOSE 8080

# Run the application
CMD ["./tusk"]

