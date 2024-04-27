# Use the official golang image as the base image
FROM golang:1.17-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download and install dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go binary
RUN go build -o main .

# Start a new stage from scratch
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /root/

# Copy the binary from the builder stage to the new stage
COPY --from=builder /app/main .

# Expose port 8008
EXPOSE 9009

# Command to run the application
CMD ["./main"]

