package main

import (
	"testing"
)

func TestMediaPath(t *testing.T) {
	system := System{
		Path: "/systems/nes",
		Name: "nes",
	}
	tests := []struct {
		path string
		want string
	}{
		{"images", "/systems/nes/images"},
		{"./images", "/systems/nes/images"},
		{"/images", "/images/nes"},
	}
	for _, tt := range tests {
		if got := system.mediaPath(tt.path); got != tt.want {
			t.Errorf("system.mediaPath(%q) = %q; want %q", tt.path, got, tt.want)
		}
	}
}
