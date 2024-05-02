FROM golang:1.22.2-alpine3.19 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod tidy
RUN go mod download
COPY . .
RUN go build -o activator .
# Break
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/activator .
EXPOSE 8080
EXPOSE 8012
EXPOSE 8013
CMD ["./activator"]

