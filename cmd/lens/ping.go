package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"
)

func icmp_ping(target string, interval float64) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	today := checkDirectory()
	filename := fmt.Sprintf("ping-%s-%s-%s-%s-%s.txt", PoP, target, INTERVAL, DURATION, datetimeString())
	filename_full := path.Join("data", today, filename)

	go func(ctx context.Context) {
		cmd := exec.CommandContext(ctx, "ping", "-D", "-c", fmt.Sprintf("%d", COUNT), "-i", fmt.Sprintf("%.2f", interval), "-I", IFACE, target)
		log.Println(cmd.String())

		f, err := os.Create(filename_full)
		if err != nil {
			log.Panic(err)
		}
		defer f.Close()

		mw := io.MultiWriter(f)

		cmd.Stdout = mw
		cmd.Stderr = mw

		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}(ctx)

	<-ctx.Done()
	if err := compress(path.Join(DATA_DIR, today), filename); err != nil {
		log.Println(err)
	}

	if ENABLE_S3 {
		upload_to_s3(filename_full+".tar.zst", path.Join(CLIENT_NAME, "ping", strconv.Itoa(time.Now().Year()), today))
	}
}

func irtt_ping() {
	ctx, cancel := context.WithTimeout(context.Background(), duration+time.Duration(time.Minute*10))
	defer cancel()

	today := checkDirectory()

	filename := fmt.Sprintf("irtt-%s-%s-%s-%s.json.gz", PoP, INTERVAL, DURATION, datetimeString())
	filename_full := path.Join("data", today, filename)

	go func(ctx context.Context) {
		var local string
		if IPVersion == 6 {
			local = fmt.Sprintf("--local=[%s]", external_ip6)
		} else {
			local = fmt.Sprintf("--local=%s", LOCAL_IP)
		}

		cmd := exec.CommandContext(ctx, "irtt", "client", fmt.Sprintf("-%d", IPVersion), "-Q", "-i", INTERVAL, "-d", DURATION, local, IRTT_HOST_PORT, "-o", filename_full)
		log.Println(cmd.String())

		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}(ctx)

	<-ctx.Done()

	if ENABLE_S3 {
		upload_to_s3(filename_full, path.Join(CLIENT_NAME, "irtt", strconv.Itoa(time.Now().Year()), today))
	}
}
