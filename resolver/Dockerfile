# Build the resolver bianry
FROM golang:1.23  AS builder
ARG TARGETOS
ARG TARGETARCH

COPY ../pkg ../pkg

WORKDIR /workspace
# Copy the Go Modules manifests
COPY ./resolver/go.mod go.mod
COPY ./resolver/go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY ./resolver/ .

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o resolver cmd/main.go


FROM alpine:latest
COPY --from=builder /workspace/resolver .
EXPOSE 8012
CMD ["./resolver"]

 