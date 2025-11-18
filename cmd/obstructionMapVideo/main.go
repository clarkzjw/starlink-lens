package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func getTimeString() string {
	return time.Now().UTC().Format("2006-01-02-15-04-05")
}

func createVideo(dataDir string, fps int) error {
	videoFile := fmt.Sprintf("%s/obstruction-map-video-%s.mp4", dataDir, getTimeString())
	cmd := exec.Command("ffmpeg",
		"-framerate", strconv.Itoa(fps),
		"-pattern_type", "glob",
		"-i", fmt.Sprintf("%s/*.png", dataDir),
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		videoFile,
	)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("failed to create video: " + err.Error() + " output: " + string(stdout))
	}
	fmt.Printf("Video created: %s\n", videoFile)
	return nil
}

func main() {
	flag.StringVar(&GRPCAddrPort, "addr_port", defaultDishAddress, "gRPC address and port of the Starlink dish")
	flag.StringVar(&Duration, "duration", "10s", "Duration for the obstruction map video")
	flag.StringVar(&DataDir, "data_dir", "./obstructionMapData", "Directory to save the obstruction map frames")
	flag.IntVar(&FPS, "fps", 10, "Frames per second for the video")
	flag.BoolVar(&CreateVideo, "video", true, "Create video from obstruction map frames")
	flag.Parse()

	if CreateVideo {
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			log.Fatal("ffmpeg is not installed. Please install ffmpeg to create videos.")
		}
	}

	fmt.Printf("Using gRPC address: %s\n", GRPCAddrPort)
	fmt.Printf("Duration for video: %s\n", Duration)

	if GRPCAddrPort == "" {
		GRPCAddrPort = defaultDishAddress
	}
	durationSecond, err := time.ParseDuration(Duration)
	if err != nil {
		fmt.Printf("Error parsing duration: %s\n", err)
		return
	}
	fmt.Printf("Duration in seconds: %.0f\n", durationSecond.Seconds())

	startTime := getTimeString()
	DataDir = fmt.Sprintf("%s/%s", DataDir, startTime)

	if _, err := os.Stat(DataDir); os.IsNotExist(err) {
		err = os.MkdirAll(DataDir, 0755)
		if err != nil {
			log.Fatalf("Error creating data directory: %s\n", err)
		}
	}

	grpcClient, err := NewGrpcClient(GRPCAddrPort)
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
		filename := fmt.Sprintf("%s/obstruction-map-%s.png", DataDir, datetime)
		fmt.Printf("Saving obstruction map to %s\n", filename)
		f, err := os.Create(filename)
		if err != nil {
			log.Println("Error creating obstruction map file: ", err)
			return
		}
		_, err = f.Write(obstructionMap.Data)
		if err != nil {
			log.Println("Error writing obstruction map: ", err)
		}
		f.Close()
		time.Sleep(time.Second * 1)
	}

	if CreateVideo {
		if err := createVideo(DataDir, FPS); err != nil {
			log.Println("Error creating video: ", err)
			return
		}
	}
}
