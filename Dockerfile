FROM golang:1.24-alpine AS builder

# Для европейских серверов используйте официальный proxy
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org

# Или отключите проверку сумм если есть проблемы с сетью
# ENV GOSUMDB=off

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o vless-manager .

FROM alpine:3.18

RUN apk update && apk add --no-cache \
    curl \
    openssl \
    sudo \
    unzip

# Устанавливаем Xray вручную
RUN ARCH=$(uname -m) && \
    case "${ARCH}" in \
    "x86_64") \
        XRAY_ARCH="64" ;; \
    "aarch64") \
        XRAY_ARCH="arm64-v8a" ;; \
    "armv7l") \
        XRAY_ARCH="arm32-v7a" ;; \
    *) \
        echo "Unsupported architecture: ${ARCH}" && exit 1 ;; \
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