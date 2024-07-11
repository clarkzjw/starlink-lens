import re
import os
import json
import config
import threading
import ipaddress
import subprocess
import netifaces as ni
from pathlib import Path
from datetime import datetime, timezone, timedelta


def check_root() -> bool:
    return os.getuid() == 0


def test_command(command: str) -> bool:
    from shutil import which
    return which(command) is not None


def check_directory() -> str:
    today = datetime.now(timezone.utc).strftime("%Y-%m-%d")
    Path("data/{}".format(today)).mkdir(parents=True, exist_ok=True)
    return today


def timestring() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%d-%H-%M-%S")


def failed(e: str) -> None:
    with open("data/failed.txt", "a+") as f:
        f.write("{}\t{}\n".format(timestring(), e))


def get_gw() -> str:
    GW = None
    if config.ACTIVE == False:
        # Inactive dish, ping IPv6 gateway only
        GW = config.IPv6_GW_Inactive
    else:
        # Active dish, ping gateway through LOCAL_IP
        external_ip = get_external_ip(6, config.IFACE)
        if ip_exist(external_ip):
            GW = get_starlink_ipv6_active_gw(config.IFACE, config.IPv6_GW_HOP)
        elif ipaddress.ip_address(config.LOCAL_IP).version == 4:
            GW = config.IPv4_GW
        else:
            GW = config.IPv6_GW_Active
    if not GW:
        exit("GW not detected")
    print("GW: ", GW)
    return GW


def get_ip_version(ip: str) -> int:
    import ipaddress
    return ipaddress.ip_address(ip).version


def get_external_ip(ipversion: int, iface: str) -> str:
    if ipversion == 6:
        result = subprocess.run(['curl', '-6', '--interface', iface, 'ipconfig.io'], capture_output=True, text=True)
    else:
        result = subprocess.run(['curl', '-4', '--interface', iface, 'ipconfig.io'], capture_output=True, text=True)

    ip_address = result.stdout.strip()
    print("external ip:", ip_address)
    return ip_address


def ip_exist(ip: str) -> bool:
    iface = get_interface(ip)
    if iface == "":
        return False
    return True


def get_interface(ip: str) -> str:
    interfaces = ni.interfaces()
    for ifce in interfaces:
        addrs4 = ni.ifaddresses(ifce).get(ni.AF_INET)
        if addrs4:
            for addr in addrs4:
                if addr['addr'] == ip:
                    return ifce
        addrs6 = ni.ifaddresses(ifce).get(ni.AF_INET6)
        if addrs6:
            for addr in addrs6:
                if addr['addr'] == ip:
                    return ifce
    return ""


def get_starlink_ipv6_active_gw(iface: str, hop: int) -> str:
    max_ttl = "2"
    cmd = ["mtr", "ipv6.google.com", "-n", "-I", iface, "-m", max_ttl, "-c", "1", "--json"]
    mtr_result = ""
    try:
        output = subprocess.check_output(cmd)
        mtr_result = json.loads(output.decode("utf-8"))
    except Exception as e:
        failed(str(e))
        return ""
    for h in mtr_result["report"]["hubs"]:
        if h["count"] == hop:
            print(h["host"])
            return h["host"]

    print("GW not detected using mtr")
    print("Trying traceroute")

    while True:
        cmd = ["traceroute", "-i", iface, "ipv6.google.com", "-n", "-m", max_ttl, "-f", max_ttl, "-q", "1"]
        traceroute_result = ""
        try:
            output = subprocess.check_output(cmd)
            traceroute_result = output.decode("utf-8")
            GW = traceroute_result.split("\n")[-2].split(" ")[3]
            if GW == "*":
                print("traceroute failed, try again...")
                continue
            return GW
        except Exception as e:
            failed(str(e))
            return ""
    return ""


def zstd(directory: str, filename: str) -> bool:
    cmd = ["tar", "--zstd", "-C", directory, "-cf", "{}/{}.tar.zst".format(directory, filename), filename, "--remove-files"]
    try:
        output = subprocess.check_output(cmd)
    except Exception as e:
        failed(str(e))
        return False
    return True


def run(func, *args, **kwargs):
    job_thread = threading.Thread(target=func, args=[kwargs.get("target"), kwargs.get("timeout")])
    job_thread.start()
