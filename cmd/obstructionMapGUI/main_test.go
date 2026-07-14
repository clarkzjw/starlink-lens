package main

import (
	"image/color"
	"testing"
	"time"
)

func TestParseInterval(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
		valid bool
	}{
		{value: "1", want: time.Second, valid: true},
		{value: " 0.5 ", want: 500 * time.Millisecond, valid: true},
		{value: "0", valid: false},
		{value: "-1", valid: false},
		{value: "abc", valid: false},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := parseInterval(test.value)
			if (err == nil) != test.valid {
				t.Fatalf("parseInterval(%q) error = %v, valid = %v", test.value, err, test.valid)
			}
			if got != test.want {
				t.Fatalf("parseInterval(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestIsResetSecond(t *testing.T) {
	for second := 0; second < 60; second++ {
		want := second == 12 || second == 27 || second == 42 || second == 57
		if got := isResetSecond(second); got != want {
			t.Fatalf("isResetSecond(%d) = %v, want %v", second, got, want)
		}
	}
}

func TestCreateImageFromSNR(t *testing.T) {
	img := createImageFromSNR(2, 2, []float32{-1, 0, 0.5, 1})
	if got := img.RGBAAt(0, 0); got != (color.RGBA{A: 255}) {
		t.Fatalf("background = %#v, want opaque black", got)
	}
	if got := img.RGBAAt(2, 1); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("zero SNR pixel = %#v, want red", got)
	}
}

func TestAccumulateUsesMinimumValidSNR(t *testing.T) {
	c := &controller{running: true, session: 1}
	first := obstructionMap{cols: 2, rows: 2, snr: []float32{-1, 0.8, 0.4, 0.2}}
	second := obstructionMap{cols: 2, rows: 2, snr: []float32{0.9, -1, 0.6, 0.1}}

	if _, current := c.accumulate(1, first); !current {
		t.Fatal("first map was rejected")
	}
	got, current := c.accumulate(1, second)
	if !current {
		t.Fatal("second map was rejected")
	}
	want := []float32{0.9, 0.8, 0.4, 0.1}
	for i := range want {
		if got.snr[i] != want[i] {
			t.Fatalf("accumulated SNR[%d] = %v, want %v", i, got.snr[i], want[i])
		}
	}
}

func TestAccumulateRejectsOldSession(t *testing.T) {
	c := &controller{running: true, session: 2}
	if _, current := c.accumulate(1, obstructionMap{cols: 1, rows: 1, snr: []float32{0.5}}); current {
		t.Fatal("map from an old session was accepted")
	}
}
