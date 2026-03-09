FROM golang:1.26 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /web-fetch-mcp ./cmd/web-fetch-mcp

FROM gcr.io/distroless/static

COPY --from=builder /web-fetch-mcp /web-fetch-mcp
ENTRYPOINT ["/web-fetch-mcp"]
