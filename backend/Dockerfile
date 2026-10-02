FROM golang:1.26 AS builder

COPY . /src
WORKDIR /src

# 国内服务器构建默认 goproxy.cn；Actions 构建经 build-arg 传官方源。
ARG GOPROXY=https://goproxy.cn
RUN make build GOPROXY=${GOPROXY}

FROM debian:stable-slim

# 腾讯云内网镜像源：国内直连 deb.debian.org 极慢/易超时
RUN sed -i "s|deb.debian.org|mirrors.cloud.tencent.com|g" /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list 2>/dev/null || true

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        curl \
		&& rm -rf /var/lib/apt/lists/ \
		&& apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /data/conf

CMD ["./server", "-conf", "/data/conf"]
