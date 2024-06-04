FROM golang:1.22.2-alpine3.19 AS builder
WORKDIR /app
COPY go.sum .
COPY go.mod .
RUN go mod tidy
COPY ../../ .
RUN go build -o activator ./cmd/activator
# Break
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/activator .
EXPOSE 8080
EXPOSE 8012
EXPOSE 8013
CMD ["./activator"]

 