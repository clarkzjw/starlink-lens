package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/phuslu/log"
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
	log.Info().Msgf("ping command: %s", cmd.String())

	f, err := os.Create(fullFilename)
	if err != nil {
		log.Error().Err(err).Msg("Error creating ping output file")
		return
	}
	defer f.Close()

	mw := io.MultiWriter(f)
	cmd.Stdout = mw
	cmd.Stderr = mw

	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Msg("Error starting ping process")
		return
	}

	log.Info().Msgf("Started ping process (PID %d) for target %s", cmd.Process.Pid, target)

	waitErr := make(chan error)
	go func() {
		waitErr <- cmd.Wait()
		close(waitErr)
	}()

	select {
	case err := <-waitErr:
		if err != nil {
			log.Error().Err(err).Msg("Ping process exited with error")
		}
	case <-ctx.Done():
		if cmd.Process != nil {
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Printf("Error sending interrupt to ping process: %v", err)
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("Error killing ping process: %v", err)
				} else {
					if err := <-waitErr; err != nil {
						log.Error().Err(err).Msg("Ping process exited with error after kill")
					}
				}
			} else {
				select {
				case err := <-waitErr:
					if err != nil {
						log.Error().Err(err).Msg("Ping process exited with error after interrupt")
					}
				case <-time.After(5 * time.Second):
					if err := cmd.Process.Kill(); err != nil {
						log.Printf("Error killing ping process: %v", err)
					}
					if err := <-waitErr; err != nil {
						log.Error().Err(err).Msg("Ping process exited with error after kill")
					}
				}
			}
		}
	}

	if err := compress(path.Join(DataDir, today), filename); err != nil {
		log.Error().Err(err).Msg("Error compressing ping output file")
		return
	}

	if EnableSwift {
		conn, err := NewSwiftConn(SwiftUsername, SwiftAPIKey, SwiftAuthURL, SwiftDomain, SwiftTenant)
		if err != nil {
			log.Error().Err(err).Msg("Error creating Swift client")
			return
		}
		localFilename := fullFilename + ".tar.zst"

		year := strconv.Itoa(time.Now().Year())
		month := fmt.Sprintf("%02d", time.Now().Month())
		day := time.Now().UTC().Format("2006-01-02")
		targetFilename := path.Join(ClientName, "ping", year, month, day, path.Base(localFilename))
		log.Info().Msgf("Uploading %s to Swift: %s", localFilename, targetFilename)

		if err := UploadToSwift(conn, SwiftContainer, localFilename, targetFilename); err != nil {
			log.Error().Err(err).Msgf("Error uploading %s to Swift container %s", localFilename, SwiftContainer)
		}
		defer func() {
			if err := os.Remove(localFilename); err != nil {
				log.Error().Err(err).Msgf("Error removing local file %s", localFilename)
			}
		}()
	}

	notify()
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
		log.Info().Msgf("irtt command: %s", cmd.String())

		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Msg("Error running irtt command")
		}
	}(ctx)

	<-ctx.Done()

	if EnableSwift {
		conn, err := NewSwiftConn(SwiftUsername, SwiftAPIKey, SwiftAuthURL, SwiftDomain, SwiftTenant)
		if err != nil {
			log.Error().Err(err).Msg("Error creating Swift client")
			return
		}
		localFilename := fullFilename

		year := strconv.Itoa(time.Now().Year())
		month := fmt.Sprintf("%02d", time.Now().Month())
		day := time.Now().UTC().Format("2006-01-02")

		targetFilename := path.Join(ClientName, "irtt", year, month, day, path.Base(localFilename))
		if err := UploadToSwift(conn, SwiftContainer, localFilename, targetFilename); err != nil {
			log.Error().Err(err).Msgf("Error uploading %s to Swift container %s", localFilename, SwiftContainer)
		}
		defer func() {
			if err := os.Remove(localFilename); err != nil {
				log.Error().Err(err).Msgf("Error removing local file %s", localFilename)
			}
		}()
	}

	notify()
}
