# engram memory MCP server. Multi-stage: build a static binary, ship it on
# distroless. Base is golang:1.26 to satisfy go.mod's `go 1.26.3`.
# Note: CI (goreleaser) builds the binary itself and injects the real version;
# this Dockerfile is the standalone/local build path (VERSION defaults to dev).
FROM golang:1.26@sha256:9d2f36f06329b2a141b9db99ffa32765cf695ee57b813ca29e245e8670bcbfff AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/engram ./cmd/engram

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/engram /engram
EXPOSE 8080
ENTRYPOINT ["/engram", "serve"]
