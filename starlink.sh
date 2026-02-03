#!/usr/bin/env bash

help () {
    echo "Usage: sudo $0 [--install | <interface>]"
    echo "  --install       Install required packages and tools"
    echo "  <interface>     Specify the Starlink network interface to use for tests"
    echo -e "\ne.g.:"
    echo "  curl -fsSL https://starlink.jinwei.me | sudo bash --install"
    echo "  curl -fsSL https://starlink.jinwei.me | sudo bash -s -- eth0"
    echo -e "\nOr run the script after downloading"
    echo "  wget -O starlink.sh https://starlink.jinwei.me"
    echo "  sudo bash starlink.sh --install"
    echo "  sudo bash starlink.sh eth0"
    echo -e "\nNote: Image display with chafa works in modern terminal emulators like iTerm2, Ghostty, etc."
    exit 1
}

INIT_FLAG=False
IFACE=""
if [ "$1" == "--install" ]; then
  INIT_FLAG=True
elif [ -n "$1" ]; then
    IFACE="$1"
    if ! ip link show "$IFACE" >/dev/null 2>&1; then
        echo "Interface $IFACE does not exist."
        exit 1
    fi
fi
if [ -z "$IFACE" ]; then
    echo "No Starlink interface specified."
    help
fi

USER_ID=$(id -u)
if [ "$USER_ID" -ne 0 ]; then
  echo "This script must be run as root. Please use sudo."
  help
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

    echo "Install additional tools..."
    apt-get install -y chafa gnuplot gawk

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
    curl -4 ipinfo.io --interface "$IFACE"
    if [ "$IPV6_AVAILABLE" -eq 0 ]; then
        curl -6 v6.ipinfo.io --interface "$IFACE"
    fi
}

cf_ray () {
    curl -sI https://www.cloudflare.com/cdn-cgi/trace | grep cf-ray
}

dns () {
    # TODO: support -b option to bind to specific interface
    OPTIONS="CHAOS TXT id.server +nsid"
    dig @1.1.1.1 $OPTIONS
    dig @8.8.8.8 $OPTIONS
    dig @9.9.9.9 $OPTIONS
}

trace () {
    OPTIONS="-r -w -i 1 -c 10 -b --mpls"
    if [ -n "$IFACE" ]; then
        OPTIONS="$OPTIONS -I $IFACE"
    fi
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
    ls -alh obstruction-map-*.png
    filename=$(ls -alh obstruction-map-* -t | head -n 1 | awk '{print $9}')
    echo "Obstruction map image saved to $filename"
    if command -v chafa >/dev/null 2>&1; then
        chafa "$filename" -f kitty -s 25x25
    fi
}

ping_gw () {
    datetime=$(date "+%y%m%d-%H%M%S")
    ping -D -I "$IFACE" -c 10000 -i 0.01 100.64.0.1 > ping-100.64.0.1-$datetime.txt
    ls -alh ping-100.64.0.1-*.txt

    filename=$(ls -alh ping-100.64.0.1-*.txt -t | head -n 1 | awk '{print $9}')
    echo "Ping 100.64.0.1 result saved to $filename"

    gawk 'BEGIN {prev_id=-1; nroll=0} $3=="bytes" {id=substr($6,10); if (prev_id-id>10000){nroll+=1}; seqid=65536*nroll+id; prev_id=id; print seqid,substr($8,6)}' "$filename" | gnuplot -e "set terminal png size 3000,500; set output '$filename.png'; unset label; unset key; plot '-'"

    chafa "$filename.png" -f kitty
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
ping_gw
