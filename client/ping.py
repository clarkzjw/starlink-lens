import time
import threading
import subprocess
from multiprocessing import Process
from datetime import datetime
from util import check_directory, failed, timestring, get_interface, zstd
import config


def icmp_ping(target: str, timeout: int) -> None:
    today = check_directory()
    print(datetime.now(), "ICMP Ping", threading.current_thread())
    icmp_ping_filename = "ping-{}-{}-{}-{}.txt".format(target, config.INTERVAL, config.DURATION, timestring())
    FILENAME = "data/{}/{}".format(today, icmp_ping_filename)

    def _do_ping():
        cmd = ["ping", "-D", "-i", "0.01", "-c", str(config.COUNT), "-I", config.IFACE, target]
        print(cmd)
        try:
            with open(FILENAME, "w") as outfile:
                subprocess.run(cmd, stdout=outfile, timeout=timeout)
        except subprocess.TimeoutExpired:
            pass

    try:
        job = Process(target=_do_ping)
        job.start()
        job.join(timeout)
        job.terminate()

        zstd("data/{}".format(today), icmp_ping_filename)

    except Exception as e:
        failed(str(e))
