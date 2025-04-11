package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"time"
)

func get_sinr(GRPC_ADDR_PORT string) {
	grpcClient, err := NewGrpcClient(GRPC_ADDR_PORT)
	if err != nil {
		log.Println("Error creating gRPC client: ", err)
		return
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()

		today := checkDirectory()

		filename := fmt.Sprintf("sinr-%s.csv", getTimeString())
		filename_full := path.Join("data", today, filename)

		go func(ctx context.Context) {
			start := time.Now().UTC()
			duration, _ = time.ParseDuration(DURATION)

			f, err := os.Create(filename_full)
			if err != nil {
				log.Panic(err)
			}
			defer f.Close()

			for time.Since(start) < duration {
				status := grpcClient.CollectDishStatus()
				sinr := status.PhyRxBeamSnrAvg

				fmt.Fprintf(f, "%d,%f\n", time.Now().UnixMilli(), sinr)
				time.Sleep(time.Millisecond * 500)
			}
		}(ctx)

		<-ctx.Done()
		if err := compress(path.Join(DATA_DIR, today), filename); err != nil {
			log.Println(err)
		}
	}
}
