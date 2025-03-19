package main

import (
	"fmt"
	"log"

	"github.com/go-co-op/gocron/v2"
)

func main() {
	checkInstalled()

	// grpcClient, err := NewGrpcClient(DishAddress)
	// if err != nil {
	// 	log.Println("Error creating gRPC client: ", err)
	// } else {
	// 	obstructionMap := grpcClient.CollectDishObstructionMap()
	// 	file := "obstruction_map.png"
	// 	f, _ := os.Create(file)
	// 	defer f.Close()

	// 	// save obstructionMap.Data of type []byte to file
	// 	f.Write(obstructionMap.Data)
	// }

	GetConfig()

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
