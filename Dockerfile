FROM debian:trixie-slim

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates bash coreutils iputils-ping mtr curl traceroute sudo iproute2 dnsutils && \
    rm -rf /var/lib/apt/lists/*

COPY lens /

CMD ["/lens"]
