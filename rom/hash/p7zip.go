package hash

import (
	"fmt"
	"path"
	"io"
	"github.com/arcane47/lzmadec"
)

func decode7Zip(f string) (io.ReadCloser, error) {
	r, err := lzmadec.NewArchive(f)
	if err != nil {
		return nil, err
	}

	for _, e := range r.Entries {
		ext := path.Ext(e.Path)
		if decoder, ok := getDecoder(ext); ok {
			rf, err := r.GetFileReader(e.Path)
			if err != nil {
				continue
			}
			rs := e.Size
			rom, err := decoder(rf, int64(rs))
			if err != nil {
				continue
			}
			return rom, nil
		}
	}
	return nil, fmt.Errorf("No valid roms found in 7zip.")
}
