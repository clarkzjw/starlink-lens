package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/clarkzjw/starlink-grpc-golang/pkg/spacex.com/api/device"
	"github.com/pbnjay/pixfont"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const grpcTimeout = 5 * time.Second

type grpcClient struct {
	conn   *grpc.ClientConn
	client device.DeviceClient
}

type obstructionMap struct {
	cols int
	rows int
	snr  []float32
}

func newGRPCClient(ctx context.Context, address string) (*grpcClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to Starlink dish gRPC interface: %w", err)
	}

	client := &grpcClient{conn: conn, client: device.NewDeviceClient(conn)}
	ctx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	resp, err := client.client.Handle(ctx, &device.Request{
		Request: &device.Request_GetDeviceInfo{},
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("gRPC GetDeviceInfo failed: %w", err)
	}
	if resp.GetGetDeviceInfo().GetDeviceInfo() == nil {
		conn.Close()
		return nil, errors.New("gRPC GetDeviceInfo failed: device info is nil")
	}

	return client, nil
}

func (c *grpcClient) close() {
	_ = c.conn.Close()
}

func (c *grpcClient) obstructionMap(ctx context.Context) (obstructionMap, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	resp, err := c.client.Handle(ctx, &device.Request{
		Request: &device.Request_DishGetObstructionMap{},
	})
	if err != nil {
		return obstructionMap{}, fmt.Errorf("gRPC GetObstructionMap failed: %w", err)
	}

	responseMap := resp.GetDishGetObstructionMap()
	if responseMap == nil {
		return obstructionMap{}, errors.New("gRPC GetObstructionMap returned no map")
	}
	rows := int(responseMap.GetNumRows())
	cols := int(responseMap.GetNumCols())
	data := responseMap.GetSnr()
	if rows <= 0 || cols <= 0 || len(data) < rows*cols {
		return obstructionMap{}, fmt.Errorf("invalid obstruction map dimensions: %dx%d with %d samples", cols, rows, len(data))
	}

	return obstructionMap{cols: cols, rows: rows, snr: data}, nil
}

func (c *grpcClient) clearObstructionMap(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	_, err := c.client.Handle(ctx, &device.Request{
		Request: &device.Request_DishClearObstructionMap{},
	})
	if err != nil {
		return fmt.Errorf("gRPC ClearObstructionMap failed: %w", err)
	}
	return nil
}

func createImageFromSNR(cols, rows int, data []float32) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, cols*2, rows*2))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	offsetX := cols / 2
	offsetY := rows / 2

	for x := range cols {
		for y := range rows {
			snr := data[y*cols+x]
			if snr > 1 {
				snr = 1
			}
			if snr >= 0 {
				level := uint8(snr * 255)
				img.Set(x+offsetX, y+offsetY, color.RGBA{R: 255, G: level, B: level, A: 255})
			}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	pixfont.DrawString(img, 10, 10, now[:10], color.White)
	pixfont.DrawString(img, 10, 20, now[11:], color.White)
	return img
}
