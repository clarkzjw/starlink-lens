package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/go-co-op/gocron/v2"
)

var (
	getObstructionMap *bool

	geoipClient *GeoIPClient
)

func init() {
	log.Println("Starlink Lens")
	getObstructionMap = flag.Bool("map", false, "Get obstruction map")

	flag.Parse()

	if *getObstructionMap {
		if DISH_GRPC_ADDR_PORT == "" {
			DISH_GRPC_ADDR_PORT = defaultDishGRPCAddress
		}
		grpcClient, err := NewGrpcClient(DISH_GRPC_ADDR_PORT)
		if err != nil {
			log.Fatal("Error creating gRPC client: ", err)
		}
		filename := fmt.Sprintf("obstruction-map-%s.png", datetimeString())
		if err := grpcClient.WriteObstructionMapImage(filename); err != nil {
			log.Fatal("Error writing obstruction map image: ", err)
		}
	}

	geoipClient = NewGeoIPClient()

	if err := LoadConfig(); err != nil {
		log.Fatal("Error loading config: ", err)
	}

	if err := CheckDeps(); err != nil {
		log.Fatal("Error checking dependency packages: ", err)
	}
}

func main() {
	if IFACE == "" {
		log.Fatal("IFACE is not set")
	}

	fmt.Printf("Starlink Gateway: %s\n", STARLINK_GATEWAY)
	fmt.Printf("DURATION: %s\n", DURATION)
	fmt.Printf("INTERVAL: %s\n", INTERVAL)
	fmt.Printf("INTERVAL_SEC: %.2f\n", INTERVAL_SEC)
	fmt.Printf("IFACE: %s\n", IFACE)
	fmt.Printf("COUNT: %d\n", COUNT)
	fmt.Printf("PoP: %s\n\n", PoP)

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
			STARLINK_GATEWAY,
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

	s.Start()

	for _, j := range s.Jobs() {
		t, _ := j.NextRun()

		fmt.Printf("[%s] Next run: %s\n", j.Name(), t)
	}

	select {}
}
