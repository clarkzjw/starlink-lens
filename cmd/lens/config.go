package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	defaultDishGRPCAddress  = "192.168.100.1:9200"
	grpcTimeout             = 5 * time.Second
	defaultIPv4CGNATGateway = "100.64.0.1"
	duration                time.Duration
	external_ip4            string
	external_ip6            string

	CLIENT_NAME      string
	STARLINK_GATEWAY string
	MANUAL_GW        string
	DURATION         string
	INTERVAL         string
	INTERVAL_SEC     float64
	IFACE            string
	COUNT            int
	ACTIVE           bool
	IPv6GWHop        string
	CRON             string
	DATA_DIR         string
	IRTT_HOST_PORT   string
	LOCAL_IP         string
	PoP              string
	IPVersion        int
	ENABLE_IRTT      = false

	DISH_GRPC_ADDR_PORT string

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

	ENABLE_SWIFT    = false
	SWIFT_USERNAME  string
	SWIFT_APIKEY    string
	SWIFT_AUTHURL   string
	SWIFT_DOMAIN    string
	SWIFT_TENANT    string
	SWIFT_CONTAINER string
)

func getConfigFromEnv() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	DISH_GRPC_ADDR_PORT = os.Getenv("DISH_GRPC_ADDR_PORT")
	if DISH_GRPC_ADDR_PORT == "" {
		DISH_GRPC_ADDR_PORT = defaultDishGRPCAddress
	}
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

	ENABLE_SWIFT = os.Getenv("ENABLE_SWIFT") == "true"
	SWIFT_USERNAME = os.Getenv("SWIFT_USERNAME")
	SWIFT_APIKEY = os.Getenv("SWIFT_APIKEY")
	SWIFT_AUTHURL = os.Getenv("SWIFT_AUTHURL")
	SWIFT_DOMAIN = os.Getenv("SWIFT_DOMAIN")
	SWIFT_TENANT = os.Getenv("SWIFT_TENANT")
	SWIFT_CONTAINER = os.Getenv("SWIFT_CONTAINER")
	return nil
}

func LoadConfig() error {
	if err := getConfigFromEnv(); err != nil {
		return err
	}

	STARLINK_GATEWAY = getGateway()
	if STARLINK_GATEWAY == "" {
		//lint:ignore ST1005 Starlink is a proper noun
		return errors.New("Starlink gateway not detected")
	}

	if ENABLE_IRTT && IRTT_HOST_PORT == "" {
		return errors.New("IRTT_HOST_PORT is not set when ENABLE_IRTT is true")
	}

	if ENABLE_IRTT && IPVersion == 4 && LOCAL_IP == "" {
		return errors.New("LOCAL_IP is not set when ENABLE_IRTT is true and IPv4 is used")
	}

	if ENABLE_SWIFT {
		if err := test_swift_connection(); err != nil {
			return fmt.Errorf("swift connection test failed: %w", err)
		}
	}

	duration, _ = time.ParseDuration(DURATION)
	interval, _ := time.ParseDuration(INTERVAL)
	COUNT = int(duration.Seconds() / (float64(interval.Microseconds()) / 1000.0 / 1000.0))
	INTERVAL_SEC = interval.Seconds()

	return nil
}
