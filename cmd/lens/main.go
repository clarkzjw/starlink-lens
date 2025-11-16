package main

import (
	"flag"
	"fmt"

	"github.com/go-co-op/gocron/v2"
	"github.com/phuslu/log"
)

var (
	getObstructionMap *bool
	geoipClient       *GeoIPClient
)

func init() {
	log.DefaultLogger.SetLevel(log.InfoLevel)

	log.Info().Msg("Starlink LENS")
	getObstructionMap = flag.Bool("map", false, "Get obstruction map")

	flag.Parse()

	if *getObstructionMap {
		if DishGrpcAddrPort == "" {
			DishGrpcAddrPort = defaultDishGRPCAddress
		}
		grpcClient, err := NewGrpcClient(DishGrpcAddrPort)
		if err != nil {
			log.Fatal().Err(err).Msg("Error creating gRPC client")
		}
		filename := fmt.Sprintf("obstruction-map-%s.png", datetimeString())
		if err := grpcClient.WriteObstructionMapImage(filename); err != nil {
			log.Fatal().Err(err).Msg("Error writing obstruction map image")
		}
		return
	}

	geoipClient = NewGeoIPClient()

	if err := LoadConfig(); err != nil {
		log.Fatal().Err(err).Msg("Error loading config")
	}

	if err := CheckDeps(); err != nil {
		log.Fatal().Err(err).Msg("Error checking dependency packages")
	}
}

func main() {
	if Iface == "" {
		log.Fatal().Msg("IFACE is not set")
	}

	log.Info().Msgf("Starlink Gateway: %s", StarlinkGateway)
	log.Info().Msgf("DURATION: %s", Duration)
	log.Info().Msgf("INTERVAL: %s", Interval)
	log.Info().Msgf("INTERVAL_SEC: %.2f", IntervalSeconds)
	log.Info().Msgf("IFACE: %s", Iface)
	log.Info().Msgf("COUNT: %d", Count)
	log.Info().Msgf("PoP: %s", PoP)

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating scheduler")
	}
	defer func() {
		if err := s.Shutdown(); err != nil {
			log.Fatal().Err(err).Msg("Error shutting down scheduler")
		}
	}()

	_, err = s.NewJob(
		gocron.CronJob(
			"30 * * * *",
			false,
		),
		gocron.NewTask(
			getGateway,
		),
	)
	if err != nil {
		log.Error().Err(err).Msg("Error creating getGateway job")
		return
	}

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
		log.Error().Err(err).Msg("Error creating icmp_ping job")
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
			log.Error().Err(err).Msg("Error creating irtt_ping job")
			return
		}
	}

	s.Start()

	for _, j := range s.Jobs() {
		t, err := j.NextRun()
		if err != nil {
			log.Warn().Err(err).Msgf("Error getting next run time for job %s", j.Name())
		}

		log.Info().Msgf("Next run for job %s: %s", j.Name(), t)
	}

	select {}
}
