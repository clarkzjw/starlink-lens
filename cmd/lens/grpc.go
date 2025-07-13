package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	device "github.com/clarkzjw/starlink-grpc-golang/pkg/spacex.com/api/device"
)

type Exporter struct {
	Conn   *grpc.ClientConn
	Client device.DeviceClient

	DishID      string
	CountryCode string
}

func NewGrpcClient(address string) (*Exporter, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to Starlink dish gRPC interface failed: %s", err.Error())
	}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
	defer cancel()

	client := device.NewDeviceClient(conn)
	resp, err := client.Handle(ctx, &device.Request{
		Request: &device.Request_GetDeviceInfo{},
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC GetDeviceInfo failed: %s", err.Error())
	}

	deviceInfo := resp.GetGetDeviceInfo().GetDeviceInfo()
	if deviceInfo == nil {
		return nil, fmt.Errorf("gRPC GetDeviceInfo failed: deviceInfo is nil")
	}

	return &Exporter{
		Conn:        conn,
		Client:      client,
		DishID:      deviceInfo.GetId(),
		CountryCode: deviceInfo.GetCountryCode(),
	}, nil
}

func (e *Exporter) CollectDishStatus() *StarlinkGetStatusResponse {
	req := &device.Request{
		Request: &device.Request_GetStatus{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
	defer cancel()
	resp, err := e.Client.Handle(ctx, req)
	if err != nil {
		fmt.Printf("gRPC GetStatus failed: %s", err.Error())
		return nil
	}

	timestamp := time.Now().Format(time.RFC3339)

	dishStatus := resp.GetDishGetStatus()
	dishStatusResp := &StarlinkGetStatusResponse{
		Timestamp:                     timestamp,
		HardwareVersion:               dishStatus.DeviceInfo.GetHardwareVersion(),
		SoftwareVersion:               dishStatus.DeviceInfo.GetSoftwareVersion(),
		CountryCode:                   dishStatus.DeviceInfo.GetCountryCode(),
		BuildID:                       dishStatus.DeviceInfo.GetBuildId(),
		DeviceUptimeSeconds:           dishStatus.DeviceState.GetUptimeS(),
		ObstructionFractionObstructed: dishStatus.ObstructionStats.GetFractionObstructed(),
		ObstructionTimeObstructed:     dishStatus.ObstructionStats.GetTimeObstructed(),
		DownlinkThroughputBps:         dishStatus.GetDownlinkThroughputBps(),
		UplinkThroughputBps:           dishStatus.GetUplinkThroughputBps(),
		PopPingLatencyMs:              dishStatus.GetPopPingLatencyMs(),
		PhyRxBeamSnrAvg:               dishStatus.GetPhyRxBeamSnrAvg(),
	}
	return dishStatusResp
}

// struct get_status response
type StarlinkGetStatusResponse struct {
	Timestamp                     string
	HardwareVersion               string
	SoftwareVersion               string
	CountryCode                   string
	BuildID                       string
	DeviceUptimeSeconds           uint64
	ObstructionFractionObstructed float32
	ObstructionTimeObstructed     float32
	DownlinkThroughputBps         float32
	UplinkThroughputBps           float32
	PopPingLatencyMs              float32
	PhyRxBeamSnrAvg               float32
}

func (e *Exporter) CollectDishObstructionMap() *StarlinkGetObstructionMapResponse {
	req := &device.Request{
		Request: &device.Request_DishGetObstructionMap{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
	defer cancel()
	resp, err := e.Client.Handle(ctx, req)
	if err != nil {
		fmt.Printf("gRPC GetObstructionMap failed: %s", err.Error())
		return nil
	}

	dishObstructionMap := resp.GetDishGetObstructionMap()
	rows := int(dishObstructionMap.NumRows)
	cols := int(dishObstructionMap.NumCols)
	referenceFrame := dishObstructionMap.GetMapReferenceFrame().String()
	data := dishObstructionMap.Snr

	upLeft := image.Point{0, 0}
	lowRight := image.Point{cols, rows}

	img := image.NewRGBA(image.Rectangle{upLeft, lowRight})

	for x := 0; x < cols; x++ {
		for y := 0; y < rows; y++ {
			snr := data[y*cols+x]
			if snr > 1 {
				// shouldn't happen
				snr = 1.0
			}
			if snr == -1 {
				// background
				img.Set(x, y, color.Black)
			} else if snr >= 0 {
				// use the same image color style as in starlink-grpc-tools
				// https://github.com/sparky8512/starlink-grpc-tools/blob/a3860e0a73d0b2280eed92eb8a2a97de0ea5fe43/dish_obstruction_map.py#L59-L87
				r := 255
				g := snr * 255
				b := snr * 255
				alpha := 255
				img.Set(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), uint8(alpha)})
			}
		}
	}

	// Encode the image to PNG format in a buffer
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		fmt.Printf("Failed to encode image: %s", err.Error())
		return nil
	}

	timestamp := time.Now().Format(time.RFC3339)
	dishObstructionMapResp := &StarlinkGetObstructionMapResponse{
		Timestamp:         timestamp,
		MapReferenceFrame: referenceFrame,
		Rows:              rows,
		Cols:              cols,
		Data:              buf.Bytes(),
	}
	return dishObstructionMapResp
}

// struct get_status response
type StarlinkGetObstructionMapResponse struct {
	Timestamp         string
	MapReferenceFrame string
	Rows              int
	Cols              int
	Data              []byte
}
