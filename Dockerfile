FROM alpine:latest

# Установите необходимые пакеты
RUN apk add --no-cache curl bash

# Установите XRay
RUN curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh | bash

# Копируем конфигурационный файл
COPY config.json /usr/local/etc/xray/config.json

# Открываем порты
EXPOSE 443 42639

# Запускаем XRay
CMD ["/usr/local/bin/xray", "run", "-c", "/usr/local/etc/xray/config.json"]
