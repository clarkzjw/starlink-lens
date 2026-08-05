package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const defaultDishAddress = "192.168.100.1:9200"

type controller struct {
	address string

	mu       sync.RWMutex
	interval string
	reset    bool
	running  bool
	cancel   context.CancelFunc
	session  uint64
	combined obstructionMap

	button           *widget.Button
	status           *widget.Label
	currentImage     *canvas.Image
	accumulatedImage *canvas.Image
}

func parseInterval(value string) (time.Duration, error) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds <= 0 {
		return 0, errorsNewInterval(value)
	}
	interval := time.Duration(seconds * float64(time.Second))
	if interval <= 0 {
		return 0, errorsNewInterval(value)
	}
	return interval, nil
}

func errorsNewInterval(value string) error {
	return fmt.Errorf("pull interval must be a number greater than zero, got %q", value)
}

func isResetSecond(second int) bool {
	return second == 12 || second == 27 || second == 42 || second == 57
}

func (c *controller) setInterval(value string) {
	c.mu.Lock()
	c.interval = value
	c.mu.Unlock()
}

func (c *controller) setReset(reset bool) {
	c.mu.Lock()
	c.reset = reset
	c.mu.Unlock()
}

func (c *controller) config() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.interval, c.reset
}

func (c *controller) start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	if _, err := parseInterval(c.interval); err != nil {
		c.mu.Unlock()
		c.setStatus(err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.running = true
	c.session++
	session := c.session
	c.combined = obstructionMap{}
	c.mu.Unlock()

	c.button.SetText("Stop")
	c.clearAccumulatedImage()
	c.setStatus("Connecting to " + c.address + "...")
	go c.run(ctx, session)
}

func (c *controller) stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.cancel()
	c.cancel = nil
	c.mu.Unlock()

	c.button.SetText("Start")
	c.setStatus("Stopped")
}

func (c *controller) toggle() {
	c.mu.RLock()
	running := c.running
	c.mu.RUnlock()
	if running {
		c.stop()
		return
	}
	c.start()
}

func (c *controller) run(ctx context.Context, session uint64) {
	client, err := newGRPCClient(ctx, c.address)
	if err != nil {
		c.runFailed(ctx, session, err)
		return
	}
	defer client.close()

	c.setStatus("Connected; polling obstruction map")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.poll(ctx, session, client)
	}()
	go func() {
		defer wg.Done()
		c.resetAtScheduledSeconds(ctx, client)
	}()
	wg.Wait()
}

func (c *controller) poll(ctx context.Context, session uint64, client *grpcClient) {
	for {
		obstructionMap, err := client.obstructionMap(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.setStatus(err.Error())
		} else {
			combined, current := c.accumulate(session, obstructionMap)
			if !current || ctx.Err() != nil {
				return
			}
			c.setImages(
				createImageFromSNR(obstructionMap.cols, obstructionMap.rows, obstructionMap.snr),
				createImageFromSNR(combined.cols, combined.rows, combined.snr),
			)
			c.setStatus("Updated " + time.Now().Format(time.RFC3339))
		}

		intervalValue, _ := c.config()
		interval, err := parseInterval(intervalValue)
		if err != nil {
			c.setStatus(err.Error())
			interval = 500 * time.Millisecond
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (c *controller) resetAtScheduledSeconds(ctx context.Context, client *grpcClient) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastReset int64 = -1

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_, reset := c.config()
			if !reset || !isResetSecond(now.Second()) {
				continue
			}
			resetKey := now.Unix()/60*60 + int64(now.Second())
			if resetKey == lastReset {
				continue
			}
			lastReset = resetKey
			if err := client.clearObstructionMap(ctx); err != nil {
				if ctx.Err() == nil {
					c.setStatus(err.Error())
				}
				continue
			}
			c.setStatus(fmt.Sprintf("Obstruction map reset at %s", now.Format(time.RFC3339)))
		}
	}
}

func (c *controller) accumulate(session uint64, current obstructionMap) (obstructionMap, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != session || !c.running {
		return obstructionMap{}, false
	}
	if c.combined.cols != current.cols || c.combined.rows != current.rows {
		c.combined = obstructionMap{
			cols: current.cols,
			rows: current.rows,
			snr:  make([]float32, current.cols*current.rows),
		}
		for i := range c.combined.snr {
			c.combined.snr[i] = -1
		}
	}
	for i, value := range current.snr[:current.cols*current.rows] {
		if value >= 0 && (c.combined.snr[i] < 0 || value < c.combined.snr[i]) {
			c.combined.snr[i] = value
		}
	}

	result := c.combined
	result.snr = append([]float32(nil), c.combined.snr...)
	return result, true
}

func (c *controller) runFailed(ctx context.Context, session uint64, err error) {
	if ctx.Err() != nil {
		return
	}
	c.mu.Lock()
	if c.session == session && c.cancel != nil {
		c.running = false
		c.cancel = nil
	}
	c.mu.Unlock()
	fyne.Do(func() {
		c.button.SetText("Start")
		c.status.SetText(err.Error())
	})
}

func (c *controller) setStatus(status string) {
	fyne.Do(func() {
		c.status.SetText(status)
	})
}

func (c *controller) setImages(current, accumulated image.Image) {
	fyne.Do(func() {
		c.currentImage.Image = current
		c.currentImage.Refresh()
		c.accumulatedImage.Image = accumulated
		c.accumulatedImage.Refresh()
	})
}

func (c *controller) clearAccumulatedImage() {
	fyne.Do(func() {
		c.accumulatedImage.Image = newBlankImage()
		c.accumulatedImage.Refresh()
	})
}

func newBlankImage() image.Image {
	data := make([]float32, 320*240)
	for i := range data {
		data[i] = -1
	}
	return createImageFromSNR(320, 240, data)
}

func main() {
	address := flag.String("addr_port", defaultDishAddress, "gRPC address and port of the Starlink dish")
	flag.Parse()

	gui := app.NewWithID("com.jinwei.starlink-lens.obstruction-map")
	window := gui.NewWindow("Starlink Obstruction Map")

	currentImage := canvas.NewImageFromImage(newBlankImage())
	currentImage.FillMode = canvas.ImageFillContain
	accumulatedImage := canvas.NewImageFromImage(newBlankImage())
	accumulatedImage.FillMode = canvas.ImageFillContain
	status := widget.NewLabel("Starting...")
	interval := widget.NewEntry()
	interval.SetText("1")
	interval.SetPlaceHolder("Seconds")
	reset := widget.NewCheck("Reset at :12, :27, :42, :57", nil)

	c := &controller{
		address:          *address,
		interval:         interval.Text,
		status:           status,
		currentImage:     currentImage,
		accumulatedImage: accumulatedImage,
	}
	button := widget.NewButton("Stop", c.toggle)
	c.button = button
	interval.OnChanged = c.setInterval
	reset.OnChanged = c.setReset

	controls := container.NewHBox(
		button,
		widget.NewLabel("Pull interval (seconds):"),
		container.NewGridWrap(fyne.NewSize(100, interval.MinSize().Height), interval),
		reset,
		layout.NewSpacer(),
	)
	figures := container.NewGridWithColumns(
		2,
		container.NewBorder(
			widget.NewLabelWithStyle("Current",
				fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil, currentImage,
		),
		container.NewBorder(widget.NewLabelWithStyle("Accumulated (minimum SNR)",
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil, accumulatedImage),
	)
	window.SetContent(container.NewBorder(controls, status, nil, nil, figures))
	window.Resize(fyne.NewSize(1200, 650))
	window.SetOnClosed(c.stop)
	c.start()
	window.ShowAndRun()
}
