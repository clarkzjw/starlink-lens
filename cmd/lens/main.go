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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gopkg.in/ini.v1"
)

var (
	GW4          string
	GW6          string
	DURATION     string
	INTERVAL     string
	INTERVAL_SEC float64
	IFACE        string
	COUNT        int
	ACTIVE       bool
	IPv6GWHop    string
	PING_CRON    string

	PoP                        string
	defaultIPv6GWHop           = "2"
	defaultIPv4CGNATGateway    = "100.64.0.1"
	defaultIPv6InactiveGateway = "fe80::200:5eff:fe00:101"
)

type MTRResult struct {
	Report struct {
		Hubs []struct {
			Count int    `json:"count"`
			Host  string `json:"host"`
		}
	}
}

func getTimeString() string {
	return time.Now().UTC().Format("2006-01-02-15-04-05")
}

func checkInstalled() {
	cmds := []string{"ping", "mtr", "traceroute", "dig", "curl", "irtt"}
	for _, c := range cmds {
		if _, err := exec.LookPath(c); err != nil {
			if _, err := os.Stat(c); err != nil {
				log.Fatalf("%s is not installed", c)
			}
		}
	}
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
	if PING_CRON, ok = os.LookupEnv("PING_CRON"); !ok {
		PING_CRON = "0 * * * *"
	}

}

func getConfigFromFile() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		os.Exit(1)
	}

	GW4 = cfg.Section("").Key("GW4").String()
	GW6 = cfg.Section("").Key("GW6").String()
	DURATION = cfg.Section("").Key("DURATION").String()
	INTERVAL = cfg.Section("").Key("INTERVAL").String()
	IFACE = cfg.Section("").Key("IFACE").String()
	ACTIVE, _ = cfg.Section("").Key("ACTIVE").Bool()
	IPv6GWHop = cfg.Section("").Key("IPv6GWHop").String()
	PING_CRON = cfg.Section("").Key("PING_CRON").String()
}

func getConfig() {
	if _, err := os.Stat("config.ini"); err == nil {
		getConfigFromFile()
	} else {
		getConfigFromEnv()
	}

	duration, _ := time.ParseDuration(DURATION)
	interval, _ := time.ParseDuration(INTERVAL)
	COUNT = int(duration.Seconds() / (float64(interval.Microseconds()) / 1000.0 / 1000.0))
	INTERVAL_SEC = interval.Seconds()

	fmt.Printf("GW4: %s\n", GW4)
	fmt.Printf("GW6: %s\n", GW6)
	fmt.Printf("DURATION: %s\n", DURATION)
	fmt.Printf("INTERVAL: %s\n", INTERVAL)
	fmt.Printf("INTERVAL_SEC: %.2f\n", INTERVAL_SEC)
	fmt.Printf("IFACE: %s\n", IFACE)
	fmt.Printf("COUNT: %d\n", COUNT)
	fmt.Println("PoP:", PoP)
}

func getExternalIP(IPVersion int) string {
	if IPVersion != 4 && IPVersion != 6 {
		IPVersion = 6
	}
	output, err := exec.Command("curl", fmt.Sprintf("-%d", IPVersion), "-m", "5", "-s", "--interface", IFACE, "ipconfig.io").CombinedOutput()
	if err != nil {
		log.Panic("get external IP failed: ", err)
	}
	return strings.Trim(string(output), "\n")
}

func getReverseDNS(ip string) string {
	cmd := exec.Command("dig", "+short", "-x", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Panic(err)
	}
	return strings.Trim(string(output), "\n")
}

func getStarlinkPoP(rdns string) string {
	// rdns: customer.sttlwax1.pop.starlinkisp.net.
	// PoP: sttlwax1

	regex := `^customer\.(?P<pop>[a-z0-9]+)\.pop\.starlinkisp\.net\.$`
	re := regexp.MustCompile(regex)
	match := re.FindStringSubmatch(rdns)
	if len(match) == 0 {
		return ""
	}
	PoP = match[1]
	return PoP
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
		cmd := exec.Command("traceroute", "-6", "-i", IFACE, "ipv6.google.com", "-n", "-m", IPv6GWHop, "-f", IPv6GWHop, "-q", "1")
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
	external_ip6 := getExternalIP(6)
	external_ip4 := getExternalIP(4)
	if IPExist(external_ip6) {
		getStarlinkPoP(getReverseDNS(external_ip6))
		return getStarlinkIPv6ActiveGateway()
	} else if net.ParseIP(external_ip4).To4() != nil {
		getStarlinkPoP(getReverseDNS(external_ip4))
		return GW4
	}
	log.Fatal("GW not detected")
	return ""
}

func icmp_ping(target string, interval float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ping", "-D", "-c", fmt.Sprintf("%d", COUNT), "-i", fmt.Sprintf("%.2f", interval), "-I", IFACE, target)

	var stdBuffer bytes.Buffer
	filename := fmt.Sprintf("ping-%s-%s-%s-%s-%s.txt", PoP, target, INTERVAL, DURATION, getTimeString())

	f, err := os.Create(filename)
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

	checkInstalled()
	getConfig()

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal("Error creating scheduler: ", err)
	}
	defer func() { _ = s.Shutdown() }()

	_, err = s.NewJob(
		gocron.CronJob(
			PING_CRON,
			false,
		),
		gocron.NewTask(
			icmp_ping,
			getGateway(),
			INTERVAL_SEC,
		),
	)
	if err != nil {
		log.Fatal("Error creating job: ", err)
	}

	s.Start()

	for _, j := range s.Jobs() {
		fmt.Println(j.NextRun())
	}

	select {}
}
