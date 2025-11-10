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
	externalIPv4            string
	externalIPv6            string

	ClientName             string
	StarlinkGateway        string
	ManualSpecifiedGateway string
	Duration               string
	Interval               string
	IntervalSeconds        float64
	Iface                  string
	Count                  int
	ActiveDish             bool
	IPv6GatewayHopCount    string
	CronString             string
	DataDir                string
	IRTTHostPort           string
	IRTTLocalIP            string
	PoP                    string
	IPVersion              int
	EnableIRTT             = false

	DishGrpcAddrPort string

	EnableSync  = false
	NotifyURL   string
	SyncServer  string
	SyncUser    string
	SyncKey     string
	SyncPath    string
	SyncCron    string
	SSHPassPath string

	EnableS3     = false
	S3Region     string
	S3Endpoint   string
	S3BucketName string
	S3AccessKey  string
	S3SecretKey  string

	EnableSwift    = false
	SwiftUsername  string
	SwiftAPIKey    string
	SwiftAuthURL   string
	SwiftDomain    string
	SwiftTenant    string
	SwiftContainer string
)

func getConfigFromEnv() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	DishGrpcAddrPort = os.Getenv("DISH_GRPC_ADDR_PORT")
	if DishGrpcAddrPort == "" {
		DishGrpcAddrPort = defaultDishGRPCAddress
	}
	ManualSpecifiedGateway = os.Getenv("MANUAL_GW")
	Duration = os.Getenv("DURATION")
	Interval = os.Getenv("INTERVAL")
	Iface = os.Getenv("IFACE")
	ActiveDish = os.Getenv("ACTIVE") == "true"
	IPv6GatewayHopCount = os.Getenv("IPv6GWHop")
	CronString = os.Getenv("CRON")
	DataDir = os.Getenv("DATA_DIR")
	EnableIRTT = os.Getenv("ENABLE_IRTT") == "true"
	IRTTHostPort = os.Getenv("IRTT_HOST_PORT")
	IRTTLocalIP = os.Getenv("LOCAL_IP")

	ClientName = os.Getenv("CLIENT_NAME")

	EnableSync = os.Getenv("ENABLE_SYNC") == "true"
	NotifyURL = os.Getenv("NOTIFY_URL")
	SyncServer = os.Getenv("SYNC_SERVER")
	SyncUser = os.Getenv("SYNC_USER")
	SyncKey = os.Getenv("SYNC_KEY")
	SyncPath = os.Getenv("SYNC_PATH")
	SyncCron = os.Getenv("SYNC_CRON")
	SSHPassPath = os.Getenv("SSHPASS_PATH")

	EnableS3 = os.Getenv("ENABLE_S3") == "true"
	S3Region = os.Getenv("S3_REGION")
	S3Endpoint = os.Getenv("S3_ENDPOINT")
	S3BucketName = os.Getenv("S3_BUCKET_NAME")
	S3AccessKey = os.Getenv("S3_ACCESS_KEY")
	S3SecretKey = os.Getenv("S3_SECRET_KEY")

	EnableSwift = os.Getenv("ENABLE_SWIFT") == "true"
	SwiftUsername = os.Getenv("SWIFT_USERNAME")
	SwiftAPIKey = os.Getenv("SWIFT_APIKEY")
	SwiftAuthURL = os.Getenv("SWIFT_AUTHURL")
	SwiftDomain = os.Getenv("SWIFT_DOMAIN")
	SwiftTenant = os.Getenv("SWIFT_TENANT")
	SwiftContainer = os.Getenv("SWIFT_CONTAINER")
	return nil
}

func LoadConfig() error {
	if err := getConfigFromEnv(); err != nil {
		return err
	}

	StarlinkGateway = getGateway()
	if StarlinkGateway == "" {
		return errors.New("gateway not detected")
	}

	if EnableIRTT && IRTTHostPort == "" {
		//nolint:revive // IRTT_HOST_PORT
		return errors.New("IRTT_HOST_PORT is not set when ENABLE_IRTT is true")
	}

	if EnableIRTT && IPVersion == 4 && IRTTLocalIP == "" {
		//nolint:revive // LOCAL_IP
		return errors.New("LOCAL_IP is not set when ENABLE_IRTT is true and IPv4 is used")
	}

	if EnableSwift {
		if err := TestSwiftConnection(); err != nil {
			return fmt.Errorf("swift connection test failed: %w", err)
		}
	}

	duration, err := time.ParseDuration(Duration)
	if err != nil {
		return fmt.Errorf("error parsing Duration: %w", err)
	}
	interval, err := time.ParseDuration(Interval)
	if err != nil {
		return fmt.Errorf("error parsing Interval: %w", err)
	}
	Count = int(duration.Seconds() / (float64(interval.Microseconds()) / 1000.0 / 1000.0))
	IntervalSeconds = interval.Seconds()

	return nil
}
