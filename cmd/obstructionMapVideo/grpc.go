package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/clarkzjw/starlink-grpc-golang/pkg/spacex.com/api/device"
	"github.com/pbnjay/pixfont"
)

var (
	defaultDishAddress = "192.168.100.1:9200"
	grpcTimeout        = 5 * time.Second
	GRPCAddrPort       string
	Duration           string
	DataDir            string
	FPS                int
	CreateVideo        bool
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
		return nil, errors.New("gRPC GetDeviceInfo failed: " + err.Error())
	}

	deviceInfo := resp.GetGetDeviceInfo().GetDeviceInfo()
	if deviceInfo == nil {
		return nil, errors.New("gRPC GetDeviceInfo failed: deviceInfo is nil")
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
	}
	return dishStatusResp
}

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
	lowRight := image.Point{cols * 2, rows * 2}

	img := image.NewRGBA(image.Rectangle{upLeft, lowRight})

	for x := 0; x < cols*2; x++ {
		for y := 0; y < rows*2; y++ {
			img.Set(x, y, color.Black)
		}
	}

	offsetX := cols / 2
	offsetY := rows / 2

	for x := 0; x < cols; x++ {
		for y := 0; y < rows; y++ {
			snr := data[y*cols+x]
			if snr > 1 {
				snr = 1.0
			}
			if snr == -1 {
				continue
			} else if snr >= 0 {
				// use the same image color style as in starlink-grpc-tools
				// https://github.com/sparky8512/starlink-grpc-tools/blob/a3860e0a73d0b2280eed92eb8a2a97de0ea5fe43/dish_obstruction_map.py#L59-L87
				r := 255
				g := snr * 255
				b := snr * 255
				alpha := 255
				img.Set(x+offsetX, y+offsetY, color.RGBA{uint8(r), uint8(g), uint8(b), uint8(alpha)})
			}
		}
	}

	timestamp := time.Now().Format(time.RFC3339Nano)
	parts := strings.Split(timestamp, "T")
	pixfont.DrawString(img, 10, 10, parts[0], color.RGBA{255, 255, 255, 255})
	pixfont.DrawString(img, 10, 20, parts[1], color.RGBA{255, 255, 255, 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		fmt.Printf("Failed to encode image: %s", err.Error())
		return nil
	}

	dishObstructionMapResp := &StarlinkGetObstructionMapResponse{
		Timestamp:         timestamp,
		MapReferenceFrame: referenceFrame,
		Rows:              rows,
		Cols:              cols,
		Data:              buf.Bytes(),
	}
	return dishObstructionMapResp
}

type StarlinkGetObstructionMapResponse struct {
	Timestamp         string
	MapReferenceFrame string
	Rows              int
	Cols              int
	Data              []byte
}
