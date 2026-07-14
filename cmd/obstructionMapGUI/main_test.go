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
