# engram memory MCP server. Multi-stage: build a static binary, ship it on
# distroless. Base is golang:1.26 to satisfy go.mod's `go 1.26.3`.
# Note: CI (goreleaser) builds the binary itself and injects the real version;
# this Dockerfile is the standalone/local build path (VERSION defaults to dev).
FROM golang:1.26@sha256:11fd8f7f63db3b6fb198797042ba4c40a4a34dc83325d3328ca3bc4bb7726786 AS build
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
