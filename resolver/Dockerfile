FROM golang:1.22  AS builder

COPY ../pkg ../pkg
WORKDIR /workspace
COPY ./resolver/go.mod go.mod
COPY ./resolver/go.sum go.sum
RUN go mod download
COPY ./resolver/ .
RUN go build -o resolver ./cmd/
# Break
FROM alpine:latest
WORKDIR /
COPY --from=builder /workspace/resolver .
EXPOSE 8080
EXPOSE 8012
EXPOSE 8013
CMD ["./resolver"]

 