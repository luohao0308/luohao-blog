FROM golang:1.26 AS builder

COPY . /src
WORKDIR /src

# 国内服务器构建默认 goproxy.cn；Actions 构建经 build-arg 传官方源。
ARG GOPROXY=https://goproxy.cn
RUN make build GOPROXY=${GOPROXY}

FROM debian:stable-slim

# apt 镜像源可注入：国内服务器构建传 mirrors.cloud.tencent.com（内网快），
# GitHub Actions 构建用默认官方源。默认 = 不改写。
ARG APT_MIRROR=deb.debian.org
RUN sed -i "s|deb.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list 2>/dev/null || true

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
