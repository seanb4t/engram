# Candidate-C memory MCP server (hl-73s.8). Multi-stage: build a static binary,
# ship it on distroless. Base is golang:1.26 to satisfy go.mod's `go 1.26.3`.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/memory-mcp .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/memory-mcp /memory-mcp
EXPOSE 8080
ENTRYPOINT ["/memory-mcp"]
