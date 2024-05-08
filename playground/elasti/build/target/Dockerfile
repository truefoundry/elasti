FROM golang:1.22.2-alpine3.19 AS builder
WORKDIR /app
COPY go.sum .
COPY go.mod .
RUN go mod tidy
RUN go mod download
COPY ../../ .
RUN go build -o target ./cmd/target
# Break
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/target .
EXPOSE 8014
CMD ["./target"]

 