package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gopkg.in/ini.v1"
)

var (
	GW4            string
	GW6            string
	DURATION       string
	INTERVAL       string
	INTERVAL_SEC   float64
	IFACE          string
	COUNT          int
	ACTIVE         bool
	IPv6GWHop      string
	CRON           string
	DATA_DIR       string
	IRTT_HOST_PORT string
	LOCAL_IP       string

	duration     time.Duration
	external_ip4 string
	external_ip6 string
	PoP          string
	IPVersion    int

	ENABLE_IRTT  = false
	ENABLE_FLENT = false

	defaultIPv6GWHop           = "2"
	defaultIPv4CGNATGateway    = "100.64.0.1"
	defaultIPv6InactiveGateway = "fe80::200:5eff:fe00:101"

	ENABLE_SYNC  = false
	CLIENT_NAME  string
	NOTIFY_URL   string
	SYNC_SERVER  string
	SYNC_USER    string
	SYNC_KEY     string
	SYNC_PATH    string
	SYNC_CRON    string
	SSHPASS_PATH string
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

func checkDirectory() string {
	today := time.Now().UTC().Format("2006-01-02")
	os.MkdirAll(path.Join("data", today), os.ModePerm)
	return today
}

func compress(directory, filename string) error {
	cmd := exec.Command("tar", "--zstd", "-C", directory, "-cf", path.Join(directory, fmt.Sprintf("%s.tar.zst", filename)), filename, "--remove-files")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
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
	if _ACTIVE, ok := os.LookupEnv("ACTIVE"); ok {
		ACTIVE, _ = strconv.ParseBool(_ACTIVE)
	}
	if IFACE, ok = os.LookupEnv("IFACE"); !ok {
		IFACE = ""
	}
	if IPv6GWHop, ok = os.LookupEnv("IPv6GWHop"); !ok {
		IPv6GWHop = defaultIPv6GWHop
	}
	if CRON, ok = os.LookupEnv("CRON"); !ok {
		CRON = "0 * * * *"
	}
	if DATA_DIR, ok = os.LookupEnv("DATA_DIR"); !ok {
		DATA_DIR = "data"
	}
	if _ENABLE_IRTT, ok := os.LookupEnv("ENABLE_IRTT"); ok {
		ENABLE_IRTT, _ = strconv.ParseBool(_ENABLE_IRTT)
	}
	if IRTT_HOST_PORT, ok = os.LookupEnv("IRTT_HOST_PORT"); !ok {
		IRTT_HOST_PORT = ""
	}
	if LOCAL_IP, ok = os.LookupEnv("LOCAL_IP"); !ok {
		LOCAL_IP = ""
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
	CRON = cfg.Section("").Key("CRON").String()
	DATA_DIR = cfg.Section("").Key("DATA_DIR").String()
	ENABLE_IRTT, _ = cfg.Section("").Key("ENABLE_IRTT").Bool()
	IRTT_HOST_PORT = cfg.Section("").Key("IRTT_HOST_PORT").String()
	LOCAL_IP = cfg.Section("").Key("LOCAL_IP").String()

	ENABLE_SYNC, _ = cfg.Section("sync").Key("ENABLE_SYNC").Bool()
	if ENABLE_SYNC {
		CLIENT_NAME = cfg.Section("sync").Key("CLIENT_NAME").String()
		NOTIFY_URL = cfg.Section("sync").Key("NOTIFY_URL").String()
		SYNC_SERVER = cfg.Section("sync").Key("SYNC_SERVER").String()
		SYNC_USER = cfg.Section("sync").Key("SYNC_USER").String()
		SYNC_KEY = cfg.Section("sync").Key("SYNC_KEY").String()
		SYNC_PATH = cfg.Section("sync").Key("SYNC_PATH").String()
		SYNC_CRON = cfg.Section("sync").Key("SYNC_CRON").String()
		SSHPASS_PATH = cfg.Section("sync").Key("SSHPASS_PATH").String()
	}
}

func getConfig() {
	if _, err := os.Stat("config.ini"); err == nil {
		getConfigFromFile()
	} else {
		getConfigFromEnv()
	}

	if IFACE == "" {
		log.Fatal("IFACE is not set")
	}

	if ENABLE_IRTT && IRTT_HOST_PORT == "" {
		log.Fatal("IRTT_HOST_PORT is not set when ENABLE_IRTT is true")
	}

	GW := getGateway()
	if ENABLE_IRTT && IPVersion == 4 && LOCAL_IP == "" {
		log.Fatal("LOCAL_IP is not set when ENABLE_IRTT is true and IPv4 is used")
	}

	duration, _ = time.ParseDuration(DURATION)
	interval, _ := time.ParseDuration(INTERVAL)
	COUNT = int(duration.Seconds() / (float64(interval.Microseconds()) / 1000.0 / 1000.0))
	INTERVAL_SEC = interval.Seconds()

	fmt.Printf("GW4: %s\n", GW4)
	fmt.Printf("GW6: %s\n", GW6)
	fmt.Printf("GW: %s\n", GW)

	fmt.Printf("DURATION: %s\n", DURATION)
	fmt.Printf("INTERVAL: %s\n", INTERVAL)
	fmt.Printf("INTERVAL_SEC: %.2f\n", INTERVAL_SEC)
	fmt.Printf("IFACE: %s\n", IFACE)
	fmt.Printf("COUNT: %d\n", COUNT)
	fmt.Printf("PoP: %s\n\n", PoP)
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
	cmd := exec.Command("dig", "@1.1.1.1", "+short", "-x", ip)
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
	return match[1]
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

func getInactiveIPv6PoP() string {
	ifces, _ := net.Interfaces()
	for _, iface := range ifces {
		if iface.Name == IFACE {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ip, _, _ := net.ParseCIDR(addr.String())
				pop := getStarlinkPoP(getReverseDNS(ip.String()))
				if pop != "" {
					return pop
				}
			}
		}
	}
	return ""
}

func getGateway() string {
	// Inactive dish, return default IPv6 inactive gateway
	// Router Bypass mode has to be set through the Starlink mobile app
	if !ACTIVE {
		PoP = getInactiveIPv6PoP()
		return defaultIPv6InactiveGateway
	}
	// Active dish, probe IPv6 active gateway through mtr or traceroute
	external_ip6 = getExternalIP(6)
	external_ip4 = getExternalIP(4)
	if IPExist(external_ip6) {
		PoP = getStarlinkPoP(getReverseDNS(external_ip6))
		IPVersion = 6
		return getStarlinkIPv6ActiveGateway()
	} else if net.ParseIP(external_ip4).To4() != nil {
		PoP = getStarlinkPoP(getReverseDNS(external_ip4))
		IPVersion = 4
		return GW4
	}
	log.Fatal("GW not detected")
	return ""
}

func icmp_ping(target string, interval float64) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	today := checkDirectory()
	filename := fmt.Sprintf("ping-%s-%s-%s-%s-%s.txt", PoP, target, INTERVAL, DURATION, getTimeString())
	filename_full := path.Join("data", today, filename)

	go func(ctx context.Context) {
		cmd := exec.CommandContext(ctx, "ping", "-D", "-c", fmt.Sprintf("%d", COUNT), "-i", fmt.Sprintf("%.2f", interval), "-I", IFACE, target)

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
}

func irtt_ping() {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	today := checkDirectory()

	filename := fmt.Sprintf("irtt-%s-%s-%s-%s.json", PoP, INTERVAL, DURATION, getTimeString())
	filename_full := path.Join("data", today, filename)

	go func(ctx context.Context) {
		var local string
		if IPVersion == 6 {
			local = fmt.Sprintf("--local=[%s]", external_ip6)
		} else {
			local = fmt.Sprintf("--local=%s", LOCAL_IP)
		}

		cmd := exec.CommandContext(ctx, "irtt", "client", fmt.Sprintf("-%d", IPVersion), "-Q", "-i", INTERVAL, "-d", DURATION, local, IRTT_HOST_PORT, "-o", filename_full)

		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}(ctx)

	<-ctx.Done()
	if err := compress(path.Join(DATA_DIR, today), filename); err != nil {
		log.Println(err)
	}
}

func sync_data() {
	cmd := exec.Command(SSHPASS_PATH,
		"-p", SYNC_KEY,
		"rsync",
		"-4",
		"--remove-source-files",
		"-e", "ssh -o StrictHostKeychecking=no",
		"--exclude=*.txt",
		"--exclude=*.json",
		"-a",
		"-v",
		"-z",
		path.Join(DATA_DIR, "*"),
		fmt.Sprintf("%s@%s:%s", SYNC_USER, SYNC_SERVER, path.Join(SYNC_PATH, CLIENT_NAME)))

	if err := cmd.Run(); err != nil {
		log.Println(err)
	}

	if NOTIFY_URL != "" {
		cmd := exec.Command("curl", "--retry", "3", "-4", "-X", "GET", NOTIFY_URL)
		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}
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
			CRON,
			false,
		),
		gocron.NewTask(
			icmp_ping,
			getGateway(),
			INTERVAL_SEC,
		),
	)
	if err != nil {
		log.Fatal("Error creating icmp_ping job: ", err)
	}

	if ENABLE_IRTT {
		_, err = s.NewJob(
			gocron.CronJob(
				CRON,
				false,
			),
			gocron.NewTask(
				irtt_ping,
			),
		)
		if err != nil {
			log.Fatal("Error creating irtt_ping job: ", err)
		}
	}

	if ENABLE_SYNC {
		_, err = s.NewJob(
			gocron.CronJob(
				SYNC_CRON,
				false,
			),
			gocron.NewTask(
				sync_data,
			),
		)
		if err != nil {
			log.Fatal("Error creating sync_data job: ", err)
		}
	}

	s.Start()

	for _, j := range s.Jobs() {
		t, _ := j.NextRun()

		fmt.Printf("[%s] Next run: %s\n", j.Name(), t)
	}

	select {}
}
