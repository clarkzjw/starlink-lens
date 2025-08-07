package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func getTimeString() string {
	return time.Now().UTC().Format("2006-01-02-15-04-05")
}

func createVideo(dataDir string, fps int) {
	videoFile := fmt.Sprintf("%s/obstruction-map-video-%s.mp4", dataDir, getTimeString())
	cmd := exec.Command("ffmpeg",
		"-framerate", fmt.Sprintf("%d", fps),
		"-pattern_type", "glob",
		"-i", fmt.Sprintf("%s/*.png", dataDir),
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		videoFile,
	)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to create video: %s\nOutput: %s", err.Error(), stdout)
		return
	}
	fmt.Printf("Video created: %s\n", videoFile)
}

func main() {
	flag.StringVar(&GRPC_ADDR_PORT, "addr_port", defaultDishAddress, "gRPC address and port of the Starlink dish")
	flag.StringVar(&DURATION, "duration", "10s", "Duration for the obstruction map video")
	flag.StringVar(&DATA_DIR, "data_dir", "./obstructionMapData", "Directory to save the obstruction map frames")
	flag.IntVar(&FPS, "fps", 10, "Frames per second for the video")
	flag.Parse()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Fatal("ffmpeg is not installed. Please install ffmpeg to create videos.")
	}

	fmt.Printf("Using gRPC address: %s\n", GRPC_ADDR_PORT)
	fmt.Printf("Duration for video: %s\n", DURATION)

	if GRPC_ADDR_PORT == "" {
		GRPC_ADDR_PORT = defaultDishAddress
	}
	durationSecond, err := time.ParseDuration(DURATION)
	if err != nil {
		fmt.Printf("Error parsing duration: %s\n", err)
		return
	}
	fmt.Printf("Duration in seconds: %.0f\n", durationSecond.Seconds())

	startTime := getTimeString()
	DATA_DIR = fmt.Sprintf("%s/%s", DATA_DIR, startTime)

	if _, err := os.Stat(DATA_DIR); os.IsNotExist(err) {
		err = os.MkdirAll(DATA_DIR, 0755)
		if err != nil {
			log.Fatalf("Error creating data directory: %s\n", err)
		}
	}

	grpcClient, err := NewGrpcClient(GRPC_ADDR_PORT)
	if err != nil {
		log.Println("Error creating gRPC client: ", err)
		return
	}

	timeNow := time.Now()
	timeEnd := timeNow.Add(durationSecond)

	for time.Now().Before(timeEnd) {
		obstructionMap := grpcClient.CollectDishObstructionMap()
		if obstructionMap == nil {
			log.Println("Failed to collect obstruction map")
			return
		}

		datetime := getTimeString()
		filename := fmt.Sprintf("%s/obstruction-map-%s.png", DATA_DIR, datetime)
		fmt.Printf("Saving obstruction map to %s\n", filename)
		f, _ := os.Create(filename)
		defer f.Close()
		_, err = f.Write(obstructionMap.Data)
		if err != nil {
			log.Println("Error writing obstruction map: ", err)
		}
		time.Sleep(time.Second * 1)
	}

	createVideo(DATA_DIR, FPS)
}
