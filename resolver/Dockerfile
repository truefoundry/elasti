FROM golang:1.22  AS builder
ARG TARGETOS
ARG TARGETARCH

COPY ../pkg ../pkg

WORKDIR /workspace
COPY ./resolver/go.mod go.mod
COPY ./resolver/go.sum go.sum
RUN go mod download
COPY ./resolver/ .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o resolver cmd/main.go

FROM alpine:latest
COPY --from=builder /workspace/resolver .

EXPOSE 8080
EXPOSE 8012
EXPOSE 8013
CMD ["./resolver"]

 