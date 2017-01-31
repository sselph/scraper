package ss

import (
	"testing"
)

func TestGetPrefix(t *testing.T) {
	tests := []struct {
		in  map[string]string
		pre string
		out string
		ok  bool
	}{
		{
			in: map[string]string{
				"media_box2d_eu": "test",
			},
			pre: "media_box2d_",
			out: "test",
			ok:  true,
		},
		{
			in: map[string]string{
				"media_box2d_eu_crc": "test",
			},
			pre: "media_box2d_",
		},
		{
			pre: "test",
		},
	}
	for _, tt := range tests {
		if got, ok := getPrefix(tt.in, tt.pre); got != tt.out || ok != tt.ok {
			t.Errorf("getPrefix(%v, %s) = (%s, %t); want (%s, %t)", tt.in, tt.pre, got, ok, tt.out, tt.ok)
		}
	}
}
