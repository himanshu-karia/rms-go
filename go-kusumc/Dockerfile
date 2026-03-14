# Stage 1: Builder
FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Copy dependency definitions first (Layer Caching)
COPY go.mod ./
RUN go mod download


# Copy Source Code
COPY . .
RUN go mod tidy

# Build the binary
# -o main: Output binary name
# ./cmd/server: The location of your main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server

# Stage 2: Runner (Alpine for debuggability)
FROM alpine:latest

# Install certificates and timezone data (Essential for Prod)
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set the working directory
WORKDIR /app

# Ensure reports directory exists
RUN mkdir -p /app/reports && chown -R appuser:appgroup /app/reports

# Copy the binary from the builder
COPY --from=builder /app/main .

# Change ownership of the binary (so appuser can execute it)
RUN chown appuser:appgroup ./main

# Switch to the non-root user
USER appuser

# Expose the port (Informational)
EXPOSE 8081

# Run
CMD ["./main"]
