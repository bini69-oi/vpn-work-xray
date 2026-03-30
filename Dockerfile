FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/vpn-productd ./cmd/vpn-productd
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/vpn-productctl ./cmd/vpn-productctl

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates iproute2 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/vpn-productd /usr/local/bin/vpn-productd
COPY --from=builder /out/vpn-productctl /usr/local/bin/vpn-productctl

ENV VPN_PRODUCT_API_TOKEN=change-me
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/vpn-productd"]
CMD ["--listen", "0.0.0.0:8080", "--data-dir", "/var/lib/vpn-product"]
