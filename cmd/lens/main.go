package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-co-op/gocron/v2"
)

func main() {
	checkInstalled()
	GetConfig()

	getObstructionMap := flag.Bool("map", false, "Get obstruction map")
	flag.Parse()

	if *getObstructionMap {
		if GRPC_ADDR_PORT == "" {
			GRPC_ADDR_PORT = defaultDishAddress
		}
		grpcClient, err := NewGrpcClient(GRPC_ADDR_PORT)
		if err != nil {
			log.Println("Error creating gRPC client: ", err)
			return
		} else {
			obstructionMap := grpcClient.CollectDishObstructionMap()
			datetime := getTimeString()
			file := fmt.Sprintf("obstruction-map-%s.png", datetime)
			f, _ := os.Create(file)
			defer f.Close()
			_, err = f.Write(obstructionMap.Data)
			if err != nil {
				log.Println("Error writing obstruction map: ", err)
			}
			return
		}
	}
	if IFACE == "" {
		log.Fatal("IFACE is not set")
	}

	fmt.Printf("GW4: %s\n", GW4)
	fmt.Printf("GW6: %s\n", GW6)
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
