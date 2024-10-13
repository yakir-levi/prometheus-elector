# Build stage
FROM --platform=$BUILDPLATFORM golang:1.21.0 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG ARCH="amd64"
ARG OS="linux"

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY api api
COPY cmd cmd
COPY config config
COPY election election
COPY health health
COPY notifier notifier
COPY readiness readiness
COPY watcher watcher
RUN echo "Building for TARGETOS=${TARGETOS} TARGETARCH=${TARGETARCH}"
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o elector-cmd ./cmd

# Final stage
FROM golang:1.21.0-alpine
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/elector-cmd ./
RUN chmod +x elector-cmd

# Run the binary
CMD ["./elector-cmd"]
