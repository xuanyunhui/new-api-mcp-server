# Build stage
FROM registry.redhat.io/ubi10/go-toolset:10.1 AS builder

WORKDIR /opt/app-root/src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /opt/app-root/bin/new-api-mcp-server ./cmd/server

# Runtime stage
FROM registry.redhat.io/ubi10-micro:10.1

COPY --from=builder /opt/app-root/bin/new-api-mcp-server /usr/local/bin/new-api-mcp-server

EXPOSE 8080 9090

USER 1001

ENTRYPOINT ["new-api-mcp-server"]
