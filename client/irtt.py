import threading
import subprocess
import ipaddress
from datetime import datetime
from util import check_directory, failed, timestring, zstd, get_external_ip, ip_exist
import config


def irtt_ping(*args, **kwargs) -> None:
    today = check_directory()
    print(datetime.now(), "IRTT Ping", threading.current_thread())
    irtt_filename = "irtt-{}-{}-{}.json".format(config.INTERVAL, config.DURATION, timestring())
    FILENAME = "data/{}/{}".format(today, irtt_filename)

    try:
        external_ip = get_external_ip(6, config.IFACE)
        if ip_exist(external_ip):
            output = subprocess.check_output(["./irtt.clarkzjw", "client", "-6", "-Q", "-i", config.INTERVAL, "-d", config.DURATION, "--local=[{}]".format(external_ip), config.IRTT_HOST_PORT, "-o", FILENAME])
        else:
            if ipaddress.ip_address(config.LOCAL_IP).version == 4:
                output = subprocess.check_output(["./irtt.clarkzjw", "client", "-4", "-Q", "-i", config.INTERVAL, "-d", config.DURATION, "--local={}".format(config.LOCAL_IP), config.IRTT_HOST_PORT, "-o", FILENAME])

        if "Error" in output.decode("utf-8"):
            failed(output.decode("utf-8"))

        zstd("data/{}".format(today), irtt_filename)

    except Exception as e:
        failed(str(e))
