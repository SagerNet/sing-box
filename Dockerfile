FROM golang:1.20-alpine AS builder
LABEL maintainer="nekohasekai <contact-git@sekai.icu>"
COPY . /go/src/github.com/sagernet/sing-box
WORKDIR /go/src/github.com/sagernet/sing-box
ARG GOPROXY=""
ENV GOPROXY ${GOPROXY}
ENV CGO_ENABLED=0
RUN set -ex \
    && apk add git build-base \
    && export COMMIT=$(git rev-parse --short HEAD) \
    && go build -v -trimpath -tags with_quic,with_wireguard,with_acme \
        -o /go/bin/sing-box \
        -ldflags "-s -w -buildid=" \
        ./cmd/sing-box
FROM alpine AS dist
LABEL maintainer="nekohasekai <contact-git@sekai.icu>"
RUN set -ex \
    && apk upgrade \
    && apk add bash tzdata ca-certificates \
    && rm -rf /var/cache/apk/*
COPY --from=builder /go/bin/sing-box /usr/local/bin/sing-box
ENTRYPOINT ["sing-box"]