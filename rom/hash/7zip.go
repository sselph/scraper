package hash

import (
	"fmt"
	"log"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"

	"github.com/kjk/lzmadec"
)

var has7z bool

func init() {
	if runtime.GOOS == "windows" {
		os.Setenv("PATH", fmt.Sprintf("C:\\Program Files\\7-zip;%s", os.Getenv("PATH")))
	}
	_, err := exec.LookPath("7z")
	has7z = err == nil
}

func decode7Zip(f string) (io.ReadCloser, error) {
	r, err := lzmadec.NewArchive(f)
	if err != nil {
		log.Printf("Skipping: %s: %q", f, err)
		return nil, err
	}
	for _, e := range r.Entries {
		ext := path.Ext(e.Path)
		if decoder, ok := getDecoder(ext); ok {
			rf, err := r.GetFileReader(e.Path)
			if err != nil {
				log.Printf("Skipping: %s: %q", e.Path, err)
				continue
			}
			rom, err := decoder(rf, e.Size)
			if err != nil {
				log.Printf("Skipping: %s: %q", e.Path, err)
				continue
			}
			log.Printf("Found: %s", e.Path)
			return rom, nil
		}
		log.Printf("Skipping: %s", e.Path)
	}
	return nil, fmt.Errorf("No valid roms found in 7zip.")
}
