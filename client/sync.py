import config
import subprocess
from pathlib import Path
from util import test_command, failed


data_dir = "./data/"
sshpass_path = "sshpass"


def rsync_data():
    try:
        cmd = "{} -p{} rsync -4 --remove-source-files -e \"ssh -o StrictHostKeychecking=no\" --exclude='*.txt' --exclude='*.json' -a -v -z {} {}@{}:{}".format(sshpass_path,
                                                                                                    config.REMOTE_PASSWD,
                                                                                                    data_dir,
                                                                                                    config.REMOTE_USER,
                                                                                                    config.REMOTE_SERVER,
                                                                                                    Path(config.REMOTE_PATH).joinpath(config.CLIENT_NAME))
        output = subprocess.check_output(cmd, shell=True)
        if "Error" in output.decode("utf-8"):
            failed(output.decode("utf-8"))

    except Exception as e:
        failed(str(e))


def scp_data():
    try:
        cmd = "{} -p{} scp -4 -r -o StrictHostKeychecking=no ./data/* {}@{}:{}".format(sshpass_path,
                                                                                    config.REMOTE_PASSWD,
                                                                                    config.REMOTE_USER,
                                                                                    config.REMOTE_SERVER,
                                                                                    Path(config.REMOTE_PATH).joinpath(config.CLIENT_NAME))
        output = subprocess.check_output(cmd, shell=True)
        if "Error" in output.decode("utf-8"):
            failed(output.decode("utf-8"))

    except Exception as e:
        failed(str(e))


def sync_data(*_):
    if test_command("rsync") == True:
        rsync_data()
    elif test_command("scp") == True:
        scp_data()
    else:
        exit("rsync and scp both not installed\n")

    output = subprocess.check_output("curl --retry 3 -v -4 -X GET {}".format(config.NOTIFY_URL), shell=True)
    print(output.decode("utf-8"))
    print("Sync data to remote storage server finished")
