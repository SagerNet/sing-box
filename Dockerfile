FROM golang:1.21-alpine AS builder
LABEL maintainer="nekohasekai <contact-git@sekai.icu>"
COPY . /go/src/github.com/sagernet/sing-box
WORKDIR /go/src/github.com/sagernet/sing-box
ARG GOPROXY=""
ARG WITH_ALL_TAGS=0
ENV GOPROXY ${GOPROXY}
ENV CGO_ENABLED=0
RUN set -ex \
    && if [ -n "$WITH_ALL_TAGS" ] && [ "$WITH_ALL_TAGS" != "0" ]; then \
        export CGO_ENABLED=1 \
        && export EXTRA_PKGS="openssl1.1-compat-dev libevent-dev zlib-dev linux-headers" \
        && export EXTRA_TAGS=",with_grpc,with_v2ray_api,with_embedded_tor,with_lwip"; \
    fi \
    && apk add git build-base $EXTRA_PKGS \
    && export COMMIT=$(git rev-parse --short HEAD) \
    && export VERSION=$(go run ./cmd/internal/read_tag) \
    && go build -v -trimpath -tags with_gvisor,with_quic,with_dhcp,with_wireguard,with_ech,with_utls,with_reality_server,with_clash_api,with_acme$EXTRA_TAGS \
        -o /go/bin/sing-box \
        -ldflags "-X \"github.com/sagernet/sing-box/constant.Version=$VERSION\" -s -w -buildid=" \
        ./cmd/sing-box
FROM alpine AS dist
LABEL maintainer="nekohasekai <contact-git@sekai.icu>"
ARG WITH_ALL_TAGS=0
RUN set -ex \
    && if [ -n "$WITH_ALL_TAGS" ] && [ "$WITH_ALL_TAGS" != "0" ]; then \
        export EXTRA_PKGS="openssl1.1-compat libevent zlib"; \
    fi \
    && apk upgrade \
    && apk add bash tzdata ca-certificates $EXTRA_PKGS \
    && rm -rf /var/cache/apk/*
COPY --from=builder /go/bin/sing-box /usr/local/bin/sing-box
ENTRYPOINT ["sing-box"]