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
		if DishGrpcAddrPort == "" {
			DishGrpcAddrPort = defaultDishGRPCAddress
		}
		grpcClient, err := NewGrpcClient(DishGrpcAddrPort)
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
	if Iface == "" {
		log.Fatal("IFACE is not set")
	}

	fmt.Printf("Starlink Gateway: %s\n", StarlinkGateway)
	fmt.Printf("DURATION: %s\n", Duration)
	fmt.Printf("INTERVAL: %s\n", Interval)
	fmt.Printf("INTERVAL_SEC: %.2f\n", IntervalSeconds)
	fmt.Printf("IFACE: %s\n", Iface)
	fmt.Printf("COUNT: %d\n", Count)
	fmt.Printf("PoP: %s\n\n", PoP)

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal("Error creating scheduler: ", err)
	}
	defer func() {
		if err := s.Shutdown(); err != nil {
			log.Printf("Error shutting down scheduler: %v", err)
		}
	}()

	_, err = s.NewJob(
		gocron.CronJob(
			CronString,
			false,
		),
		gocron.NewTask(
			ICMPPing,
			StarlinkGateway,
			IntervalSeconds,
		),
	)
	if err != nil {
		log.Printf("Error creating icmp_ping job: %s", err.Error())
		return
	}

	if EnableIRTT {
		_, err = s.NewJob(
			gocron.CronJob(
				CronString,
				false,
			),
			gocron.NewTask(
				IRTTPing,
			),
		)
		if err != nil {
			log.Printf("Error creating irtt_ping job: %s", err.Error())
			return
		}
	}

	s.Start()

	for _, j := range s.Jobs() {
		t, err := j.NextRun()
		if err != nil {
			log.Printf("Error getting next run time for job %s: %s", j.Name(), err.Error())
		}

		fmt.Printf("[%s] Next run: %s\n", j.Name(), t)
	}

	select {}
}
