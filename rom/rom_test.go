package rom

import (
	"path/filepath"
	"testing"
)

func TestFixPath(t *testing.T) {
	tests := []struct {
		sep                    rune
		xml, local, path, want string
	}{
		{'/', "./art/wheel", "./art/wheel", "art/wheel/test.png", "./art/wheel/test.png"},
		{'/', "art/wheel", "art/wheel", "art/wheel/test.png", "./art/wheel/test.png"},
		{'/', "image", "image", "image/test.png", "./image/test.png"},
		{'/', "image", "/home/sselph/image", "/home/sselph/image/test.png", "./image/test.png"},
		{'/', "/image", "/home/sselph/image", "/home/sselph/image/test.png", "/image/test.png"},
		{'\\', "image", `C:\image`, `C:\image\test.png`, "./image/test.png"},
		{'\\', `image`, `image`, `image\test.png`, "./image/test.png"},
		{'\\', `/image`, `.\image`, `image\test.png`, "/image/test.png"},
	}
	for _, tt := range tests {
		if filepath.Separator != tt.sep {
			continue
		}
		if got := fixPath(tt.xml, tt.local, tt.path); got != tt.want {
			t.Errorf("fixPath(%q, %q, %q) = %q want %q", tt.xml, tt.local, tt.path, got, tt.want)
		}
	}
}
