#!/usr/bin/env bash

INIT_FLAG=False
if [ "$1" == "--install" ]; then
  INIT_FLAG=True
fi

USER_ID=$(id -u)
if [ "$USER_ID" -ne 0 ]; then
  echo "This script must be run as root. Please use sudo."
  exit 1
fi

install () {
    OS="$(uname -s)"
    echo "Detected OS: $OS"
    if [ "$OS" != "Linux" ]; then
        echo "This script only supports Linux."
        exit 1
    fi

    DISTRO="$(. /etc/os-release && echo "$ID")"
    echo "Detected distribution: $DISTRO"

    if [ "$DISTRO" != "ubuntu" ] && [ "$DISTRO" != "debian" ]; then
        echo "This script only supports Ubuntu and Debian."
        exit 1
    fi

    ARCH="$(dpkg --print-architecture)"
    echo "Detected architecture: $ARCH"

    echo "Updating package lists..."
    apt-get update

    echo "Installing essential packages..."
    apt-get install -y curl gnupg2 ca-certificates lsb-release traceroute mtr iputils-ping screen jq bind9-dnsutils wget

    echo "Importing GPG key..."
    curl -fsSL https://pkg.jinwei.me/clarkzjw-pkg.key | tee /etc/apt/keyrings/clarkzjw-pkg.asc

    echo "Adding Lens repository..."
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/clarkzjw-pkg.asc] https://pkg.jinwei.me/lens any main" | tee /etc/apt/sources.list.d/starlink-lens.list

    echo "Adding Starlink-Telegraf repository..."
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/clarkzjw-pkg.asc] https://pkg.jinwei.me/starlink-telegraf any main" | tee /etc/apt/sources.list.d/starlink-telegraf.list

    apt-get update && apt-get install lens starlink-telegraf -y

    GRPCURL_VERSION=$(curl -s "https://api.github.com/repos/fullstorydev/grpcurl/tags" | jq -r '.[0].name')
    GRPCURL_VERSION="${GRPCURL_VERSION#v}"
    echo "Latest grpcurl version: $GRPCURL_VERSION"
    GRPCURL_PKG_URL=https://github.com/fullstorydev/grpcurl/releases/download/v"$GRPCURL_VERSION"/grpcurl_"$GRPCURL_VERSION"_linux_"$ARCH".deb
    echo "Downloading grpcurl from $GRPCURL_PKG_URL..."
    wget -O /tmp/grpcurl.deb "$GRPCURL_PKG_URL"
    dpkg -i /tmp/grpcurl.deb && rm -f /tmp/grpcurl.deb
    grpcurl --version

    echo "Installing speedtest-cli..."
    curl -s https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh | bash
    apt-get install speedtest -y
}

test_ipv6 () {
    curl -6 -s https://one.one.one.one >/dev/null
    return $?
}

IPV6_AVAILABLE=$(test_ipv6; echo $?)

geoip () {
    curl -4 ipinfo.io
    if [ "$IPV6_AVAILABLE" -eq 0 ]; then
        curl -6 v6.ipinfo.io
    fi
}

cf_ray () {
    curl -sI https://www.cloudflare.com/cdn-cgi/trace | grep cf-ray
}

dns () {
    OPTIONS="CHAOS TXT id.server +nsid"
    dig @1.1.1.1 $OPTIONS
    dig @8.8.8.8 $OPTIONS
    dig @9.9.9.9 $OPTIONS
}

trace () {
    OPTIONS="-r -w -i 1 -c 10 -b --mpls"
    mtr 1.1.1.1 $OPTIONS
    if [ "$IPV6_AVAILABLE" -eq 0 ]; then
        mtr 2606:4700:4700::1111 $OPTIONS
    fi
}

grpc_status () {
    grpcurl -plaintext -d {\"get_status\":{}} 192.168.100.1:9200 SpaceX.API.Device.Device/Handle
    grpcurl -plaintext -d {\"get_location\":{}} 192.168.100.1:9200 SpaceX.API.Device.Device/Handle
}

networking () {
    ip addr show
    ip route show
}

obstruction_map () {
    lens -map
    ls -alh
}

if [ "$INIT_FLAG" == "True" ]; then
    install
    exit 0
fi

set -x
networking
grpc_status
geoip
cf_ray
dns
trace
obstruction_map
