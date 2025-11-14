FROM debian:trixie-slim

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends zstd ca-certificates bash coreutils iputils-ping mtr curl traceroute sudo iproute2 dnsutils && \
    rm -rf /var/lib/apt/lists/*

ENV IRTT_VERSION=0.9.1-clarkzjw

RUN curl -o starlink-irtt_${IRTT_VERSION}_linux_amd64.deb -fsSL https://github.com/clarkzjw/irtt/releases/download/${IRTT_VERSION}/starlink-irtt_${IRTT_VERSION}_linux_amd64.deb && \
    dpkg -i ./starlink-irtt_${IRTT_VERSION}_linux_amd64.deb && \
    rm starlink-irtt_${IRTT_VERSION}_linux_amd64.deb

COPY lens /

CMD ["/lens"]
