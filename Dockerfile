# 可通过 --build-arg GO_IMAGE=... / RUNTIME_IMAGE=... 切换基础镜像。
ARG GO_IMAGE=golang:1.26.2-alpine
ARG RUNTIME_IMAGE=alpine:3.23

FROM ${GO_IMAGE} AS builder

# 默认使用阿里云镜像源；在网络环境不同的情况下可通过 --build-arg 覆盖。
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ARG GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata

WORKDIR /src

# 先下载依赖，使依赖层能被 Docker 缓存复用。
COPY go.mod go.sum ./
RUN GOPROXY=${GOPROXY} go mod download

COPY . .
RUN CGO_ENABLED=0 GOPROXY=${GOPROXY} go build -trimpath -ldflags="-s -w" -o /out/wormhole-server ./cmd/server

FROM ${RUNTIME_IMAGE} AS runtime

ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine

RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app -h /app app

WORKDIR /app
COPY --from=builder /out/wormhole-server ./wormhole-server

USER app
EXPOSE 8080

ENTRYPOINT ["./wormhole-server"]
