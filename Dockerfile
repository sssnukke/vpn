FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o vless-manager .

FROM alpine:3.18

RUN apk add --no-cache \
    xray \
    openssl \
    curl \
    sudo

RUN adduser -D -u 1000 vlessuser

COPY --from=builder /app/vless-manager /usr/local/bin/
COPY --from=builder /app/config.yaml /etc/vless-manager/
COPY scripts/init-xray.sh /usr/local/bin/

RUN mkdir -p /etc/xray /var/log/xray \
    && chown -R vlessuser:vlessuser /etc/xray /var/log/xray

RUN echo "vlessuser ALL=(root) NOPASSWD: /usr/bin/systemctl reload xray" >> /etc/sudoers

EXPOSE 8080 443

CMD ["sh", "-c", "/usr/local/bin/init-xray.sh && /usr/local/bin/vless-manager"]