# engram memory MCP server. Multi-stage: build a static binary, ship it on
# distroless. Base is golang:1.26 to satisfy go.mod's `go 1.26.3`.
# Note: CI (goreleaser) builds the binary itself and injects the real version;
# this Dockerfile is the standalone/local build path (VERSION defaults to dev).
FROM golang:1.26@sha256:792443b89f65105abba56b9bd5e97f680a80074ac62fc844a584212f8c8102c3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/engram ./cmd/engram

FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
COPY --from=build /out/engram /engram
EXPOSE 8080
ENTRYPOINT ["/engram", "serve"]
