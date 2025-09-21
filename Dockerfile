FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o vless-manager .

FROM alpine:3.18

# Меняем mirror на Cloudflare (надёжный вариант)
RUN sed -i 's|dl-cdn.alpinelinux.org|dl-cdn.alpinelinux.org|g' /etc/apk/repositories \
    && echo "https://mirror.clarkson.edu/alpine/v3.18/main" >> /etc/apk/repositories \
    && echo "https://mirror.clarkson.edu/alpine/v3.18/community" >> /etc/apk/repositories

# Устанавливаем пакеты
RUN apk add --no-cache \
    curl \
    openssl \
    sudo \
    unzip

# Определяем архитектуру и устанавливаем Xray
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
    "amd64")   XRAY_ARCH="64" ;; \
    "arm64")   XRAY_ARCH="arm64-v8a" ;; \
    "arm")     XRAY_ARCH="arm32-v7a" ;; \
    *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac && \
    curl -L "https://github.com/XTLS/Xray-core/releases/download/v1.8.11/Xray-linux-${XRAY_ARCH}.zip" -o xray.zip && \
    unzip xray.zip xray -d /usr/local/bin/ && \
    rm xray.zip && \
    chmod +x /usr/local/bin/xray

RUN adduser -D -u 1000 vlessuser

COPY --from=builder /app/vless-manager /usr/local/bin/
COPY --from=builder /app/config.yaml /etc/vless-manager/
COPY scripts/init-xray.sh /usr/local/bin/

RUN mkdir -p /etc/xray /var/log/xray \
    && chown -R vlessuser:vlessuser /etc/xray /var/log/xray \
    && chmod +x /usr/local/bin/init-xray.sh /usr/local/bin/vless-manager

RUN echo "vlessuser ALL=(root) NOPASSWD: /usr/bin/rc-service xray reload" >> /etc/sudoers

EXPOSE 8080 443

CMD ["sh", "-c", "/usr/local/bin/init-xray.sh && /usr/local/bin/vless-manager"]
