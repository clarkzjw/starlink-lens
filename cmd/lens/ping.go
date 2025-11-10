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
	ctx, cancel := context.WithTimeout(context.Background(), duration)

	today := checkDirectory()
	var filename string
	if PoP != "" {
		filename = fmt.Sprintf("ping-%s-%s-%s-%s-%s.txt", PoP, target, Interval, Duration, datetimeString())
	} else {
		filename = fmt.Sprintf("ping-%s-%s-%s-%s.txt", target, Interval, Duration, datetimeString())
	}
	fullFilename := path.Join("data", today, filename)

	// channels to communicate process and exit
	procCh := make(chan *os.Process, 1)
	doneCh := make(chan struct{})

	go func() {
		// ensure context is cancelled when goroutine (process) finishes
		defer cancel()

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

		// start the process (do not tie to ctx so we can send signals manually)
		if err := cmd.Start(); err != nil {
			log.Println("Error starting ping process: ", err)
			return
		}

		// publish process to caller
		procCh <- cmd.Process

		// wait for process to exit
		if err := cmd.Wait(); err != nil {
			log.Println(err)
		}

		// signal done
		close(doneCh)
	}()

	// wait for either process start or context done
	select {
	case <-ctx.Done():
		// context finished before process was published or already done; try to see if proc arrived
		select {
		case proc := <-procCh:
			if proc != nil {
				// try polite interrupt first
				if err := proc.Signal(os.Interrupt); err != nil {
					log.Printf("Error sending interrupt to ping process: %v", err)
				} else {
					// give process some time to exit
					select {
					case <-doneCh:
						// exited gracefully
					case <-time.After(5 * time.Second):
						// still running: kill
						if err := proc.Kill(); err != nil {
							log.Printf("Error killing ping process: %v", err)
						}
					}
				}
			}
		default:
			// no process to signal
		}
	case proc := <-procCh:
		// process started; now wait for ctx done
		<-ctx.Done()
		// attempt graceful shutdown
		if proc != nil {
			if err := proc.Signal(os.Interrupt); err != nil {
				log.Printf("Error sending interrupt to ping process: %v", err)
			} else {
				select {
				case <-doneCh:
					// exited
				case <-time.After(5 * time.Second):
					if err := proc.Kill(); err != nil {
						log.Printf("Error killing ping process: %v", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), duration+time.Minute*10)

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
