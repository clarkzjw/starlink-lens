import time
import schedule
from util import parse_delta, run, check_root, test_command

import config
from ping import icmp_ping


START = ":00"

IPS = []
with open("ips.txt", "r") as f:
    for line in f:
        print(line.strip())
        IPS.append(line.strip())

for ip in IPS:
    schedule.every(1).hours.at(START).do(run, icmp_ping, target=ip, timeout=parse_delta(config.DURATION).seconds)


if __name__ == "__main__":
    if check_root() == False:
        exit("You need to have root priviledges to run this script\n")
    
    for cmd in ["ping", "zstd"]:
        if test_command(cmd) == False:
            exit("{} not installed\n".format(cmd))
    
    print("Interval: ", parse_delta(config.INTERVAL))
    print("Duration: ", parse_delta(config.DURATION))
    print("Extra ICMP Targets: ", config.EXTRA_ICMP_HOSTS)
        
    # schedule.run_all()
    print("\nNext run", schedule.next_run())
    while True:
        schedule.run_pending()
        time.sleep(0.5)
