import time
import threading
import ipaddress
import subprocess

from datetime import datetime

import config
from util import check_directory, failed, timestring, zstd, get_external_ip, ip_exist


available_tests = [
    "cubic_bbr",
    "cubic_reno",
    "ping",
    "rrul",
    "rrul_be",
    "rrul_icmp",
    "tcp_bidirectional",
    "tcp_download",
    "tcp_upload",
    "tcp_2down",
    "tcp_4down",
    "tcp_2up",
    "tcp_4up",
    "tcp_2up_delay",
    "tcp_4up_squarewave"
]

def flent_test(*args, **kwargs) -> None:
    today = check_directory()
    print(datetime.now(), "Flent Test", threading.current_thread())
    FILENAME = "data/{}".format(today)

    try:
        for test in available_tests:
            external_ip = get_external_ip(6, config.IFACE)
            output = subprocess.check_output(["flent", test, "-4", "-l", config.FLENT_DURATION, "-s", "0.01", "-H", config.FLENT_SERVER, "-D", FILENAME])

            if "Error" in output.decode("utf-8"):
                failed(output.decode("utf-8"))

            time.sleep(30)

    except Exception as e:
        failed(str(e))
