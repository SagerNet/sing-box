FROM golang:1.19-alpine AS builder
LABEL maintainer="nekohasekai <contact-git@sekai.icu>"
COPY . /go/src/github.com/sagernet/sing-box
WORKDIR /go/src/github.com/sagernet/sing-box
ARG GOPROXY=""
ENV GOPROXY ${GOPROXY}
ENV CGO_ENABLED=0
RUN set -ex \
    && apk add git build-base \
    && export COMMIT=$(git rev-parse --short HEAD) \
    && go build -v -trimpath -tags 'no_gvisor,with_quic,with_wireguard,with_acme' \
        -o /go/bin/sing-box \
        -ldflags "-X github.com/sagernet/sing-box/constant.Commit=${COMMIT} -w -s -buildid=" \
        ./cmd/sing-box
FROM alpine AS dist
LABEL maintainer="nekohasekai <contact-git@sekai.icu>"
RUN [ ! -e /etc/nsswitch.conf ] && echo 'hosts: files dns' > /etc/nsswitch.conf
RUN set -ex \
    && apk upgrade \
    && apk add bash tzdata ca-certificates \
    && rm -rf /var/cache/apk/*
COPY --from=builder /go/bin/sing-box /usr/local/bin/sing-box
ENTRYPOINT ["sing-box"]