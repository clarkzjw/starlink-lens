import os
import re
from datetime import datetime, timezone, timedelta


def parse_delta(delta):
    """ Parses a human readable timedelta (3d5h19m) into a datetime.timedelta.
    Delta includes:
    * Xd days
    * Xh hours
    * Xm minutes
    * Xms milliseconds
    Values can be negative following timedelta's rules. Eg: -5h-30m
    """
    # https://gist.github.com/santiagobasulto/698f0ff660968200f873a2f9d1c4113c
    TIMEDELTA_REGEX = (r'((?P<days>-?\d+)d)?'
                    r'((?P<hours>-?\d+)h)?'
                    r'((?P<milliseconds>-?\d+)ms)?'
                    r'((?P<minutes>-?\d+)m)?')
    TIMEDELTA_PATTERN = re.compile(TIMEDELTA_REGEX, re.IGNORECASE)

    match = TIMEDELTA_PATTERN.match(delta)
    if match:
        parts = {k: int(v) for k, v in match.groupdict().items() if v}
        return timedelta(**parts)


INTERVAL = os.getenv("INTERVAL", "10ms")
DURATION = os.getenv("DURATION", "60m")
DURATION_SECONDS = str(parse_delta(DURATION).seconds)
COUNT = int(parse_delta(DURATION).seconds / (parse_delta(INTERVAL).microseconds/1000.0/1000.0))

IRTT_HOST_PORT = os.getenv("IRTT_HOST_PORT")

IFACE = os.getenv("IFACE", "<iface>")
LOCAL_IP = os.getenv("LOCAL_IP")
IPv4_GW = os.getenv("GW4", "100.64.0.1")
IPv6_GW_Active = os.getenv("GW6")

# check with
# TERM=vt220 mtr ipv6.google.com -n -I enp2s0 -c 1
IPv6_GW_HOP = 2
IPv6_GW_Inactive = os.getenv("GW6_INACTIVE", "fe80::200:5eff:fe00:101")
ACTIVE = os.getenv("ACTIVE", "True").lower() in ('true', '1', 't')
STARLINK_IP = os.getenv("STARLINK_IP", "34.83.112.196")

EXTRA_ICMP_HOSTS = os.getenv("EXTRA", "").split(",")

# Sync data to remote storage server
NOTIFY_URL = os.getenv("NOTIFY_URL", "<notify url>")
ACTIVE_SYNC = os.getenv("ACTIVE_SYNC", "True").lower() in ('true', '1', 't')
REMOTE_SERVER = os.getenv("SYNC_SERVER", "<sync server>")
REMOTE_USER = os.getenv("SYNC_USER", "<sync user>")
REMOTE_PASSWD = os.getenv("SYNC_PASSWD", "<password>")
REMOTE_PATH = os.getenv("SYNC_PATH", "<remote storage path>")
CLIENT_NAME = os.getenv("CLIENT_NAME", "<client name>")

# Flent, netperf
ENABLE_FLENT = os.getenv("ENABLE_FLENT", "False").lower() in ('true', '1', 't')
FLENT_SERVER = os.getenv("FLENT_SERVER", "<flent server>")
FLENT_DURATION = os.getenv("FLENT_DURATION", "120")
