package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
)

var (
	GW4                        string
	GW6                        string
	DURATION                   string
	INTERVAL                   string
	IFACE                      string
	COUNT                      int
	ACTIVE                     bool
	IPv6GWHop                  string
	defaultIPv6GWHop           = "2"
	defaultIPv4CGNATGateway    = "100.64.0.1"
	defaultIPv6InactiveGateway = "fe80::200:5eff:fe00:101"
)

func getConfigFromEnv() {
	var ok bool
	if GW4, ok = os.LookupEnv("GW4"); !ok {
		GW4 = defaultIPv4CGNATGateway
	}
	if GW6, ok = os.LookupEnv("GW6"); !ok {
		GW6 = defaultIPv6InactiveGateway
	}
	if DURATION, ok = os.LookupEnv("DURATION"); !ok {
		DURATION = "1h"
	}
	if INTERVAL, ok = os.LookupEnv("INTERVAL"); !ok {
		INTERVAL = "10ms"
	}
	if _ACTIVE, ok := os.LookupEnv("ACTIVE"); !ok {
		log.Fatal("Dish status ACTIVE is not set")
	} else {
		ACTIVE, _ = strconv.ParseBool(_ACTIVE)
	}
	if IFACE, ok = os.LookupEnv("IFACE"); !ok {
		log.Fatal("IFACE is not set")
	}
	if IPv6GWHop, ok = os.LookupEnv("IPv6GWHop"); !ok {
		IPv6GWHop = defaultIPv6GWHop
	}

	duration, _ := time.ParseDuration(DURATION)
	interval, _ := time.ParseDuration(INTERVAL)
	COUNT = int(duration.Seconds() / (float64(interval.Microseconds()) / 1000.0 / 1000.0))
}

func getExternalIP(IPVersion int) string {
	if IPVersion != 4 && IPVersion != 6 {
		IPVersion = 6
	}
	output, err := exec.Command("curl", fmt.Sprintf("-%d", IPVersion), "-m", "5", "-s", "--interface", IFACE, "ipconfig.io").CombinedOutput()
	if err != nil {
		log.Panic(err)
	}
	return strings.Trim(string(output), "\n")
}

func IPExist(ip string) bool {
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To16() != nil {
				if ipnet.IP.To16().String() == ip {
					return true
				}
			}
		}
	}
	return false
}

type MTRResult struct {
	Report struct {
		Hubs []struct {
			Count int    `json:"count"`
			Host  string `json:"host"`
		}
	}
}

func getStarlinkIPv6ActiveGateway() string {
	fmt.Println("Getting Starlink IPv6 active gateway")
	cmd, err := exec.Command("mtr", "ipv6.google.com", "-n", "-m", IPv6GWHop, "-I", IFACE, "-c", "1", "--json").CombinedOutput()
	if err != nil {
		log.Panic(err)
	}

	var mtrOutput MTRResult
	json.Unmarshal([]byte(string(cmd)), &mtrOutput)

	for _, h := range mtrOutput.Report.Hubs {
		if strconv.Itoa(h.Count) == IPv6GWHop {
			return h.Host
		}
	}

	fmt.Println("GW not detected using mtr")
	fmt.Println("Trying traceroute")

	for {
		cmd := exec.Command("traceroute", "-6", "-i", IFACE, "www.google.com", "-n", "-m", IPv6GWHop, "-f", IPv6GWHop, "-q", "1")
		tracerouteResult := ""
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Panic(err)
		}
		tracerouteResult = string(output)
		GW := strings.Split(tracerouteResult, "\n")[len(strings.Split(tracerouteResult, "\n"))-2]
		if GW == "*" {
			fmt.Println("traceroute failed, try again...")
			continue
		}
		return GW
	}
}

func getGateway() string {
	// Inactive dish, return default IPv6 inactive gateway
	// Router Bypass mode has to be set through the Starlink mobile app
	if !ACTIVE {
		return defaultIPv6InactiveGateway
	}
	// Active dish, probe IPv6 active gateway through mtr or traceroute
	external_ip := getExternalIP(6)
	if IPExist(external_ip) {
		return getStarlinkIPv6ActiveGateway()
	} else if net.ParseIP(external_ip).To4() != nil {
		return GW4
	}
	log.Fatal("GW not detected")
	return ""
}

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

func main() {
	getConfigFromEnv()

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
