package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
)

func sync_data() {
	cmd := exec.Command(SSHPASS_PATH,
		"-p", SYNC_KEY,
		"rsync",
		"-4",
		"--remove-source-files",
		"-e", "'ssh -o StrictHostKeychecking=no'",
		"--exclude=*.txt",
		"--exclude=*.json",
		"-a",
		"-v",
		"-z",
		path.Join(DATA_DIR, "*"),
		fmt.Sprintf("%s@%s:%s", SYNC_USER, SYNC_SERVER, path.Join(SYNC_PATH, CLIENT_NAME)))

	shellCmd := exec.Command("bash", "-c", cmd.String())
	log.Println(shellCmd.String())

	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	if err := shellCmd.Run(); err != nil {
		log.Println(err)
	}

	if NOTIFY_URL != "" {
		cmd := exec.Command("curl", "--retry", "3", "-4", "-X", "GET", NOTIFY_URL)
		log.Println(cmd.String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}
}
