# Starlink LENS ![Build](https://github.com/clarkzjw/starlink-lens/actions/workflows/build.yaml/badge.svg)

See information about tracerouting Starlink backbone map in the [backbone-map](./backbone-map) directory.

**Work In Progress**

This tool is used to collect Starlink user terminal (UT) to gateway latency data with `ping` and `irtt`, for the [LENS](https://github.com/clarkzjw/LENS) dataset.

## Install

* Ubuntu 22.04 and newer: Check out PPA [clarkzjw/lens](https://launchpad.net/~clarkzjw/+archive/ubuntu/lens)

* Ubuntu 20.04: Download the deb package from [Releases](https://github.com/clarkzjw/starlink-lens/releases), and install it with `sudo apt install ./starlink-lens-0.1.0-1-amd64.deb`

Dependencies installed: https://github.com/clarkzjw/starlink-lens/blob/master/cmd/lens/.fpm#L8-L20

## Configuration

`/opt/lens/config.ini`:

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

`/usr/lib/systemd/system/lens.service`:

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

```bash
sudo systemctl daemon-reload
sudo systemctl enable lens.service
sudo systemctl start lens.service
```

## TODO

- [ ] Integrate with Starlink gRPC interface
