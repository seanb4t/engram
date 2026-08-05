# engram memory MCP server. Multi-stage: build a static binary, ship it on
# distroless. Base is golang:1.26 to satisfy go.mod's `go 1.26.3`.
# Note: CI (goreleaser) builds the binary itself and injects the real version;
# this Dockerfile is the standalone/local build path (VERSION defaults to dev).
FROM golang:1.26@sha256:b004b9c35c68c8bcf5420e5164423073a1de5f0c7d4b9121784780ceb7f9961f AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/engram ./cmd/engram

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35
COPY --from=build /out/engram /engram
EXPOSE 8080
ENTRYPOINT ["/engram", "serve"]
