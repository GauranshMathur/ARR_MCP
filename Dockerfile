# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src

# Dependencies first so source edits do not invalidate the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
# CGO_ENABLED=0 produces a static binary, which is what lets the final stage be
# a distroless image with no libc.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X github.com/GauranshMathur/ARR_MCP/pkg/server.Version=${VERSION}" \
    -o /out/arr-mcp ./cmd/arr-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/arr-mcp /usr/local/bin/arr-mcp

# Runs as uid 65532 from the base image; never as root.
USER nonroot:nonroot
EXPOSE 8080

# Default to HTTP for container deployments. For stdio, override the command:
#   docker run -i --rm --env-file .env ghcr.io/gauranshmathur/arr-mcp --transport stdio
ENTRYPOINT ["/usr/local/bin/arr-mcp"]
CMD ["--transport", "http", "--addr", "0.0.0.0:8080"]
