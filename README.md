# Starlink LENS ![Build](https://github.com/clarkzjw/starlink-lens/actions/workflows/build.yaml/badge.svg)

**Work In Progress**

This tool is used to collect Starlink user terminal (UT) to gateway latency data with `ping` and `irtt`, for the [LENS](https://github.com/clarkzjw/LENS) dataset.

## Install

Pre-built binaries are available for Debian-based Linux systems. You can download prebuilt `.deb` packages from GitHub [releases](https://github.com/clarkzjw/starlink-lens/releases) page or install via `apt`:

Install the prerequisites:

```bash
sudo apt-get install curl gnupg2 ca-certificates lsb-release
```

Import the GPG key:

```bash
curl -fsSL https://pkg.jinwei.me/clarkzjw-pkg.key | sudo tee /etc/apt/keyrings/clarkzjw-pkg.asc
```

Verify that the downloaded file contains the proper key:

```bash
gpg --dry-run --quiet --no-keyring --import --import-options import-show /etc/apt/keyrings/clarkzjw-pkg.asc
```

The output should contain the full fingerprint `84A174C0FB90CE887F6F319A5B3DE76F745C39FE` as follows:

```
pub   rsa4096 2025-03-18 [SC] [expires: 2030-03-17]
      84A174C0FB90CE887F6F319A5B3DE76F745C39FE
uid                      clarkzjw (GPG Key for Packages on Cloudflare R2) <pkg@jinwei.me>
sub   rsa4096 2025-03-18 [E] [expires: 2030-03-17]
sub   rsa4096 2025-03-18 [S] [expires: 2030-03-17]
```

Set up the repository:

```bash
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/clarkzjw-pkg.asc] https://pkg.jinwei.me/lens $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/clarkzjw-pkg.list
```

Update the package list and install:

```bash
sudo apt-get update
sudo apt-get install -y lens
```

## Usage

### Latency Measurement Configuration

Create a configuration file at `/opt/lens/config.ini` with the following content:

```ini
GW4 = "100.64.0.1"
GW6 = "fe80::200:5eff:fe00:101"
DURATION = "1h"
INTERVAL = "10ms"
IFACE = "xxx"
ACTIVE = true
IPv6GWHop = "2"
CRON = "0 * * * *"
DATA_DIR = "data"
ENABLE_IRTT = False
IRTT_HOST_PORT = ""

[sync]
ENABLE_SYNC = True
CLIENT_NAME = xxx
NOTIFY_URL = ""
SYNC_SERVER = ""
SYNC_USER = "lens"
SYNC_KEY = ""
SYNC_PATH = "/home/lens/data/"
SYNC_CRON = "30 * * * *"
SSHPASS_PATH = "sshpass"
```

Note:

+ `100.64.0.1` is the default IPv4 gateway for most Starlink users. `fe80::200:5eff:fe00:101` is the ICMP-reachable IPv6 gateway for inactive Starlink users.
+ If you have active Starlink subscription, you can get your Starlink IPv6 gateway by running `mtr -6 ipv6.google.com` and looking for the second hop.

### One-shot obstruction map

The `lens` command provides an alternative to the Python-based [`starlink-grpc-tools`](https://github.com/sparky8512/starlink-grpc-tools) to obtain the UT obstruction map.

```bash
./lens -h
Usage of ./lens:
  -map
        Get obstruction map
```

By default, it uses the default Starlink gRPC address `192.168.100.1:9200`. In rare cases, if custom gRPC address is needed (e.g., the client has multiple NICs connected to different Starlink dishes), one can set a `GRPC_ADDR_PORT` variable in `config.ini`.

```ini
GRPC_ADDR_PORT = "192.168.2.1:1234" # custom gRPC address and port
```

It saves the obstruction map in png format in the current directory with the naming scheme `obstruction-map-2025-03-19-23-20-51.png`. It follows the same color scheme as starlink-grpc-tools, with the only difference is changing the background to black from transparent.

![](./static/obstruction-map-2025-03-20-00-24-53.png)

### SINR Measurement

This firmware feature has been removed by Starlink.

~~Since Starlink dish firmware [`2025.04.08.cr53207`](https://github.com/clarkzjw/starlink-grpc-golang/commit/b26a153763dbf8c84dcd3b54c4fda0a3a084e5b7), gRPC method `get_status` returns `PhyRxBeamSnrAvg`.~~
~~To capture continuous SINR measurement, one can set `ENABLE_SINR = True` in `config.ini`.~~

### Complete Configuration Example

```ini
GW4 = "100.64.0.1"
GW6 = "fe80::200:5eff:fe00:101"
DURATION = "1h"
INTERVAL = "10ms"
IFACE = "xxx"
ACTIVE = true
IPv6GWHop = "2"
CRON = "0 * * * *"
DATA_DIR = "data"
ENABLE_IRTT = False
IRTT_HOST_PORT = ""
ENABLE_SINR = False
GRPC_ADDR_PORT = "192.168.100.1:9200"

[sync]
ENABLE_SYNC = True
CLIENT_NAME = xxx
NOTIFY_URL = ""
SYNC_SERVER = ""
SYNC_USER = "lens"
SYNC_KEY = ""
SYNC_PATH = "/home/lens/data/"
SYNC_CRON = "30 * * * *"
SSHPASS_PATH = "sshpass"
```

## TODO

- [ ] Support measurement data upload via S3 compatible endpoints
