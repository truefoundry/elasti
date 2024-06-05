FROM golang:1.22  AS builder
WORKDIR /app
COPY ../pkg ../pkg
COPY ./resolver/go.sum .
COPY ./resolver/go.mod .
RUN go mod tidy
COPY ../../ .
RUN go build -o resolver ./resolver/cmd/resolver
# Break
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/resolver .
EXPOSE 8080
EXPOSE 8012
EXPOSE 8013
CMD ["./resolver"]

 