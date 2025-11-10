package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"
)

func ICMPPing(target string, interval float64) {
	ctx, cancel := context.WithTimeout(context.Background(), sessionDuration)
	defer cancel()

	today := checkDirectory()
	var filename string
	if PoP != "" {
		filename = fmt.Sprintf("ping-%s-%s-%s-%s-%s.txt", PoP, target, Interval, Duration, datetimeString())
	} else {
		filename = fmt.Sprintf("ping-%s-%s-%s-%s.txt", target, Interval, Duration, datetimeString())
	}
	fullFilename := path.Join("data", today, filename)

	cmd := exec.Command("ping", "-D", "-c", strconv.Itoa(Count), "-i", fmt.Sprintf("%.2f", interval), "-I", Iface, target)
	log.Println(cmd.String())

	f, err := os.Create(fullFilename)
	if err != nil {
		log.Println("Error creating ping output file: ", err)
		return
	}
	defer f.Close()

	mw := io.MultiWriter(f)
	cmd.Stdout = mw
	cmd.Stderr = mw

	if err := cmd.Start(); err != nil {
		log.Println("Error starting ping process: ", err)
		return
	}

	fmt.Printf("Started ping process (PID %d) for target %s\n", cmd.Process.Pid, target)

	waitErr := make(chan error)
	go func() {
		waitErr <- cmd.Wait()
		close(waitErr)
	}()

	select {
	case err := <-waitErr:
		if err != nil {
			log.Println(err)
		}
	case <-ctx.Done():
		if cmd.Process != nil {
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Printf("Error sending interrupt to ping process: %v", err)
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("Error killing ping process: %v", err)
				} else {
					if err := <-waitErr; err != nil {
						log.Println(err)
					}
				}
			} else {
				select {
				case err := <-waitErr:
					if err != nil {
						log.Println(err)
					}
				case <-time.After(5 * time.Second):
					if err := cmd.Process.Kill(); err != nil {
						log.Printf("Error killing ping process: %v", err)
					}
					if err := <-waitErr; err != nil {
						log.Println(err)
					}
				}
			}
		}
	}

	if err := compress(path.Join(DataDir, today), filename); err != nil {
		log.Println(err)
	}

	if EnableSwift {
		conn, err := NewSwiftConn(SwiftUsername, SwiftAPIKey, SwiftAuthURL, SwiftDomain, SwiftTenant)
		if err != nil {
			log.Println("Error creating Swift client: ", err)
			return
		}
		localFilename := fullFilename + ".tar.zst"

		year := strconv.Itoa(time.Now().Year())
		month := fmt.Sprintf("%02d", time.Now().Month())
		day := time.Now().UTC().Format("2006-01-02")
		targetFilename := path.Join(ClientName, "ping", year, month, day, path.Base(localFilename))
		fmt.Printf("Uploading to Swift: %s\n", targetFilename)

		if err := UploadToSwift(conn, SwiftContainer, localFilename, targetFilename); err != nil {
			log.Println("Error uploading to Swift: ", err)
		}
		defer func() {
			if err := os.Remove(localFilename); err != nil {
				log.Println("Error removing local file: ", err)
			}
		}()
	}
}

func IRTTPing() {
	ctx, cancel := context.WithTimeout(context.Background(), sessionDuration+time.Minute*10)

	today := checkDirectory()

	filename := fmt.Sprintf("irtt-%s-%s-%s-%s.json.gz", PoP, Interval, Duration, datetimeString())
	fullFilename := path.Join("data", today, filename)

	go func(ctx context.Context) {
		defer cancel()

		var local string
		if IPVersion == 6 {
			local = fmt.Sprintf("--local=[%s]", externalIPv6)
		} else {
			local = fmt.Sprintf("--local=%s", IRTTLocalIP)
		}

		cmd := exec.CommandContext(ctx,
			"irtt",
			"client",
			fmt.Sprintf("-%d", IPVersion),
			"-Q",
			"-i", Interval,
			"-d", Duration,
			local,
			IRTTHostPort,
			"-o", fullFilename)
		log.Println(cmd.String())

		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}(ctx)

	<-ctx.Done()

	if EnableSwift {
		conn, err := NewSwiftConn(SwiftUsername, SwiftAPIKey, SwiftAuthURL, SwiftDomain, ClientName)
		if err != nil {
			log.Println("Error creating Swift client: ", err)
			return
		}
		localFilename := fullFilename + ".tar.zst"

		year := strconv.Itoa(time.Now().Year())
		month := fmt.Sprintf("%02d", time.Now().Month())
		day := time.Now().UTC().Format("2006-01-02")

		targetFilename := path.Join(ClientName, "irtt", year, month, day, path.Base(localFilename))
		if err := UploadToSwift(conn, SwiftContainer, localFilename, targetFilename); err != nil {
			log.Println("Error uploading to Swift: ", err)
		}
		defer func() {
			if err := os.Remove(localFilename); err != nil {
				log.Println("Error removing local file: ", err)
			}
		}()
	}
}
