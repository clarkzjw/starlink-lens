import time
import schedule
from sync import sync_data
from util import run, check_root, test_command, get_gw

import config
from irtt import irtt_ping
from ping import icmp_ping
from flent import flent_test


START = ":00"
if config.ACTIVE_SYNC:
    schedule.every(1).hours.at(":30").do(run, sync_data)

if config.ENABLE_FLENT:
    schedule.every(1).hours.at(START).do(run, flent_test)

if config.ACTIVE == True:
    schedule.every(1).hours.at(START).do(run, irtt_ping)
else:
    schedule.every(1).hours.at(START).do(run, icmp_ping, target=config.STARLINK_IP, timeout=config.parse_delta(config.DURATION).seconds)

# Ping Starlink gateway
schedule.every(1).hours.at(START).do(run, icmp_ping, target=get_gw(), timeout=config.parse_delta(config.DURATION).seconds)

# Extra ICMP targets
for extra in config.EXTRA_ICMP_HOSTS:
    if len(extra) > 0:
        schedule.every(1).hours.at(START).do(run, icmp_ping, target=extra, timeout=config.parse_delta(config.DURATION).seconds)


if __name__ == "__main__":
    #if check_root() == False:
    #    exit("You need to have root priviledges to run this script\n")

    for cmd in ["./irtt.clarkzjw", "ping", "zstd", "mtr"]:
        if test_command(cmd) == False:
            exit("{} not installed\n".format(cmd))

    print("Interval: ", config.parse_delta(config.INTERVAL))
    print("Duration: ", config.parse_delta(config.DURATION))
    print("Active: ", config.ACTIVE)
    print("Gateway: ", get_gw())
    print("IRTT Target: ", config.IRTT_HOST_PORT)
    print("Extra ICMP Targets: ", config.EXTRA_ICMP_HOSTS)
    print("NOTIFY_URL: ", config.NOTIFY_URL)
    if get_gw() == None:
        exit("\nGet gateway failed\n")
    if config.ACTIVE and config.IRTT_HOST_PORT == None:
        exit("\nYou have to set IRTT_HOST_PORT for active dish\n")

    #schedule.run_all()
    print("\nNext run", schedule.next_run())
    while True:
        schedule.run_pending()
        time.sleep(0.5)
