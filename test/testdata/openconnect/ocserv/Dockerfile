FROM debian:trixie-slim@sha256:cedb1ef40439206b673ee8b33a46a03a0c9fa90bf3732f54704f99cb061d2c5a

ARG OCSERV_VERSION=1.3.0-2

RUN apt-get update \
    && apt-get install --yes --no-install-recommends ocserv="${OCSERV_VERSION}" iproute2 python3 \
    && rm -rf /var/lib/apt/lists/*

COPY echo_server.py /usr/local/bin/openconnect-echo-server

EXPOSE 443/tcp 443/udp
