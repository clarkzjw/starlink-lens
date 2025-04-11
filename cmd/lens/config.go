package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/ini.v1"
)

var (
	configFilePath             = "/opt/lens/config.ini"
	defaultDishAddress         = "192.168.100.1:9200"
	grpcTimeout                = 5 * time.Second
	defaultIPv6GWHop           = "2"
	defaultIPv4CGNATGateway    = "100.64.0.1"
	defaultIPv6InactiveGateway = "fe80::200:5eff:fe00:101"
	duration                   time.Duration
	external_ip4               string
	external_ip6               string

	GRPC_ADDR_PORT string
	GW4            string
	GW6            string
	MANUAL_GW      string
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
	PoP            string
	IPVersion      int
	ENABLE_IRTT    = false
	ENABLE_FLENT   = false
	ENABLE_SYNC    = false
	ENABLE_SINR    = false
	CLIENT_NAME    string
	NOTIFY_URL     string
	SYNC_SERVER    string
	SYNC_USER      string
	SYNC_KEY       string
	SYNC_PATH      string
	SYNC_CRON      string
	SSHPASS_PATH   string
)

// deprecated: use getConfigFromFile instead
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
	if _ENABLE_SINR, ok := os.LookupEnv("ENABLE_SINR"); ok {
		ENABLE_SINR, _ = strconv.ParseBool(_ENABLE_SINR)
	}
}

func getConfigFromFile() {
	cfg, err := ini.Load(configFilePath)
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		os.Exit(1)
	}

	GRPC_ADDR_PORT = cfg.Section("").Key("GRPC_ADDR_PORT").String()
	GW4 = cfg.Section("").Key("GW4").String()
	GW6 = cfg.Section("").Key("GW6").String()
	MANUAL_GW = cfg.Section("").Key("MANUAL_GW").String()
	DURATION = cfg.Section("").Key("DURATION").String()
	INTERVAL = cfg.Section("").Key("INTERVAL").String()
	IFACE = cfg.Section("").Key("IFACE").String()
	ACTIVE = cfg.Section("").Key("ACTIVE").MustBool()
	IPv6GWHop = cfg.Section("").Key("IPv6GWHop").String()
	CRON = cfg.Section("").Key("CRON").String()
	DATA_DIR = cfg.Section("").Key("DATA_DIR").String()
	ENABLE_IRTT = cfg.Section("").Key("ENABLE_IRTT").MustBool()
	IRTT_HOST_PORT = cfg.Section("").Key("IRTT_HOST_PORT").String()
	LOCAL_IP = cfg.Section("").Key("LOCAL_IP").String()

	ENABLE_SYNC = cfg.Section("sync").Key("ENABLE_SYNC").MustBool()
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

	ENABLE_SINR = cfg.Section("").Key("ENABLE_SINR").MustBool()
}

func GetConfig() {
	if _, err := os.Stat(configFilePath); err == nil {
		getConfigFromFile()
	} else {
		getConfigFromEnv()
	}

	if ENABLE_IRTT && IRTT_HOST_PORT == "" {
		log.Fatal("IRTT_HOST_PORT is not set when ENABLE_IRTT is true")
	}

	var GW string
	if MANUAL_GW != "" {
		GW4 = MANUAL_GW
		GW6 = MANUAL_GW
	} else {
		GW = getGateway()
		if GW == "" {
			log.Fatal("GW not detected")
		}
		fmt.Println("GW: ", GW)
	}

	if ENABLE_IRTT && IPVersion == 4 && LOCAL_IP == "" {
		log.Fatal("LOCAL_IP is not set when ENABLE_IRTT is true and IPv4 is used")
	}

	duration, _ = time.ParseDuration(DURATION)
	interval, _ := time.ParseDuration(INTERVAL)
	COUNT = int(duration.Seconds() / (float64(interval.Microseconds()) / 1000.0 / 1000.0))
	INTERVAL_SEC = interval.Seconds()
}
