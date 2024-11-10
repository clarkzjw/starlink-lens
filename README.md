# Starlink LENS ![Build](https://github.com/clarkzjw/starlink-lens/actions/workflows/build.yaml/badge.svg)

See information about tracerouting Starlink backbone in the [backbone-map](./backbone-map) directory.

---

**Work In Progress**

This tool is used to collect Starlink user terminal (UT) to gateway latency data with `ping` and `irtt`, for the [LENS](https://github.com/clarkzjw/LENS) dataset.

## Install

Pre-built binaries are available for Debian-based Linux systems on `x86_64` and `arm64`. However, you can also build from source with `Golang` in the `cmd/lens` directory.

* Ubuntu 20.04 and newer: Download the pre-built deb package for your architecture from [Releases](https://github.com/clarkzjw/starlink-lens/releases), and install it with `sudo apt install ./starlink-lens-<arch>.deb`

Dependencies installed: https://github.com/clarkzjw/starlink-lens/blob/master/cmd/lens/.fpm-amd64#L8-L18

## Configuration

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

If you want to start the service with systemd, create a systemd service file at `/usr/lib/systemd/system/lens.service`:

```
[Unit]
Description=Starlink LENS dataset

[Service]
Type=simple
User=root
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
WorkingDirectory=/opt/lens
ExecStart=/usr/bin/lens

[Install]
WantedBy=default.target
```

Then run the following commands to enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable lens.service
sudo systemctl start lens.service
```

## TODO

- [ ] Integrate with Starlink gRPC interface
