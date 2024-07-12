package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// COUNT = int(parse_delta(DURATION).seconds / (parse_delta(INTERVAL).microseconds/1000.0/1000.0))
func icmp(target, iface string, interval float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ping", "-D", "-c", "100", target)

	var stdBuffer bytes.Buffer
	file := "ping.log"
	f, err := os.Create(file)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	mw := io.MultiWriter(f, os.Stdout, &stdBuffer)

	cmd.Stdout = mw
	cmd.Stderr = mw

	if err := cmd.Run(); err != nil {
		log.Panic(err)
	}

	log.Println(stdBuffer.String())
}

var (
	IPv4GW string = "100.64.0.1"
	IPv6GW string = "fe80::200:5eff:fe00:101"
)

func main() {
	s, err := gocron.NewScheduler()
	if err != nil {
		fmt.Println("Error creating scheduler")
	}
	defer func() { _ = s.Shutdown() }()

	_, err = s.NewJob(
		gocron.CronJob(
			"* * * * *",
			false,
		),
		gocron.NewTask(
			icmp,
			"1.1.1.1",
			"en0",
			0.01,
		),
	)
	if err != nil {
		fmt.Println("Error creating job")
	}
	s.Start()

	select {}
}
