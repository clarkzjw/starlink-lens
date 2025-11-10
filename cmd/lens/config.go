package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	defaultDishAddress         = "192.168.100.1:9200"
	grpcTimeout                = 5 * time.Second
	defaultIPv4CGNATGateway    = "100.64.0.1"
	defaultIPv6InactiveGateway = "fe80::200:5eff:fe00:101"
	duration                   time.Duration
	external_ip4               string
	external_ip6               string

	CLIENT_NAME    string
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

	GRPC_ADDR_PORT string

	ENABLE_SYNC  = false
	NOTIFY_URL   string
	SYNC_SERVER  string
	SYNC_USER    string
	SYNC_KEY     string
	SYNC_PATH    string
	SYNC_CRON    string
	SSHPASS_PATH string

	ENABLE_S3      = false
	S3_REGION      string
	S3_ENDPOINT    string
	S3_BUCKET_NAME string
	S3_ACCESS_KEY  string
	S3_SECRET_KEY  string
)

func getConfigFromEnv() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("error loading .env file: %v", err)
	}

	GRPC_ADDR_PORT = os.Getenv("GRPC_ADDR_PORT")
	if GRPC_ADDR_PORT == "" {
		GRPC_ADDR_PORT = defaultDishAddress
	}
	GW4 = os.Getenv("GW4")
	GW6 = os.Getenv("GW6")
	MANUAL_GW = os.Getenv("MANUAL_GW")
	DURATION = os.Getenv("DURATION")
	INTERVAL = os.Getenv("INTERVAL")
	IFACE = os.Getenv("IFACE")
	ACTIVE = os.Getenv("ACTIVE") == "true"
	IPv6GWHop = os.Getenv("IPv6GWHop")
	CRON = os.Getenv("CRON")
	DATA_DIR = os.Getenv("DATA_DIR")
	ENABLE_IRTT = os.Getenv("ENABLE_IRTT") == "true"
	IRTT_HOST_PORT = os.Getenv("IRTT_HOST_PORT")
	LOCAL_IP = os.Getenv("LOCAL_IP")

	CLIENT_NAME = os.Getenv("CLIENT_NAME")

	ENABLE_SYNC = os.Getenv("ENABLE_SYNC") == "true"
	NOTIFY_URL = os.Getenv("NOTIFY_URL")
	SYNC_SERVER = os.Getenv("SYNC_SERVER")
	SYNC_USER = os.Getenv("SYNC_USER")
	SYNC_KEY = os.Getenv("SYNC_KEY")
	SYNC_PATH = os.Getenv("SYNC_PATH")
	SYNC_CRON = os.Getenv("SYNC_CRON")
	SSHPASS_PATH = os.Getenv("SSHPASS_PATH")

	ENABLE_S3 = os.Getenv("ENABLE_S3") == "true"
	S3_REGION = os.Getenv("S3_REGION")
	S3_ENDPOINT = os.Getenv("S3_ENDPOINT")
	S3_BUCKET_NAME = os.Getenv("S3_BUCKET_NAME")
	S3_ACCESS_KEY = os.Getenv("S3_ACCESS_KEY")
	S3_SECRET_KEY = os.Getenv("S3_SECRET_KEY")

	return nil
}

func GetConfig() {
	if err := getConfigFromEnv(); err != nil {
		log.Fatal(err)
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
	}

	if ENABLE_IRTT && IPVersion == 4 && LOCAL_IP == "" {
		log.Fatal("LOCAL_IP is not set when ENABLE_IRTT is true and IPv4 is used")
	}

	duration, _ = time.ParseDuration(DURATION)
	interval, _ := time.ParseDuration(INTERVAL)
	COUNT = int(duration.Seconds() / (float64(interval.Microseconds()) / 1000.0 / 1000.0))
	INTERVAL_SEC = interval.Seconds()
}
