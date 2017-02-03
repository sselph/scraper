package testdata

import (
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Data struct {
	Dir   string
	Files []File
}

func (d *Data) Close() error {
	return os.RemoveAll(d.Dir)
}

func New() (*Data, error) {
	var err error
	data := &Data{}
	dir, err := ioutil.TempDir("", "roms")
	if err != nil {
		return data, err
	}
	data.Dir = dir
	defer func() {
		if err != nil {
			data.Close()
		}
	}()
	// Bin Extensions
	binFile := make([]byte, 512)
	binHash := "5c3eb80066420002bc3dcc7ca4ab6efad7ed4ae5"
	binExts := []string{
		".bin", ".a26", ".rom", ".cue", ".gdi", ".gb", ".gba",
		".gbc", ".lyx", ".32x", ".gg", ".pce", ".sms", ".sg",
		".col", ".int", ".ngp", ".ngc", ".vb", ".vec", ".gam",
		".a78", ".j64", ".jag", ".lnx", ".mgw", ".nds", ".fds",
	}
	for _, e := range binExts {
		p := filepath.Join(dir, fmt.Sprintf("test%s", e))
		err = ioutil.WriteFile(p, binFile, 0777)
		if err != nil {
			return data, err
		}
		data.Files = append(data.Files, File{Path: p, SHA1: binHash})
	}

	// Lynx
	lnxFile := make([]byte, 64)
	copy(lnxFile, []byte("LYNX"))
	lnxFile = append(lnxFile, binFile...)
	lnxPath := filepath.Join(dir, "test.lnx")
	lyxPath := filepath.Join(dir, "test.lyx")
	err = ioutil.WriteFile(lnxPath, lnxFile, 0777)
	if err != nil {
		return data, err
	}
	err = ioutil.WriteFile(lyxPath, lnxFile, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: lnxPath, SHA1: binHash})
	data.Files = append(data.Files, File{Path: lyxPath, SHA1: binHash})

	// A7800
	a78File := make([]byte, 128)
	copy(a78File, []byte(" ATARI7800"))
	a78File = append(a78File, binFile...)
	a78Path := filepath.Join(dir, "a7800.a78")
	err = ioutil.WriteFile(a78Path, a78File, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: a78Path, SHA1: binHash})

	// N64
	v64File := make([]byte, 1024)
	n64File := make([]byte, 1024)
	z64File := make([]byte, 1024)
	copy(v64File, []byte{0, 0x80, 0, 0})
	copy(n64File, []byte{0, 0, 0, 0x80})
	copy(z64File, []byte{0x80, 0, 0, 0})
	for i := 4; i < 1024; i = i + 4 {
		v64File[i], z64File[i+1], n64File[i+2] = 1, 1, 1
		v64File[i+1], z64File[i], n64File[i+3] = 2, 2, 2
		v64File[i+2], z64File[i+3], n64File[i] = 3, 3, 3
		v64File[i+3], z64File[i+2], n64File[i+1] = 4, 4, 4
	}
	v64Path := filepath.Join(dir, "test-v64.v64")
	n64Path := filepath.Join(dir, "test-n64.v64")
	z64Path := filepath.Join(dir, "test-z64.v64")
	err = ioutil.WriteFile(v64Path, v64File, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: v64Path, SHA1: "00ba552537f953776b37a05230e9f1c2f6d4c145"})
	err = ioutil.WriteFile(n64Path, n64File, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: n64Path, SHA1: "00ba552537f953776b37a05230e9f1c2f6d4c145"})
	err = ioutil.WriteFile(z64Path, z64File, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: z64Path, SHA1: "00ba552537f953776b37a05230e9f1c2f6d4c145"})

	//Bad N64
	v64Path = filepath.Join(dir, "test-bad-v64.v64")
	n64Path = filepath.Join(dir, "test-bad-n64.v64")
	z64Path = filepath.Join(dir, "test-bad-z64.v64")
	err = ioutil.WriteFile(v64Path, []byte{0, 0x80, 0, 0, 0, 0}, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: v64Path, SHA1: "a5d06af4902696ab97fd92747bc7886c990dfed5"})
	err = ioutil.WriteFile(n64Path, []byte{0, 0, 0, 0x80, 0, 0}, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: n64Path, SHA1: "a5d06af4902696ab97fd92747bc7886c990dfed5"})
	err = ioutil.WriteFile(z64Path, []byte{0x80, 0, 0, 0, 0, 0}, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: z64Path, SHA1: "a5d06af4902696ab97fd92747bc7886c990dfed5"})

	// SNES
	snesFile1 := make([]byte, 1024)
	snesFile2 := make([]byte, 1536)
	snesPath1 := filepath.Join(dir, "test.smc")
	snesPath2 := filepath.Join(dir, "test.sfc")
	err = ioutil.WriteFile(snesPath1, snesFile1, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: snesPath1, SHA1: "60cacbf3d72e1e7834203da608037b1bf83b40e8"})
	err = ioutil.WriteFile(snesPath2, snesFile2, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: snesPath2, SHA1: "60cacbf3d72e1e7834203da608037b1bf83b40e8"})

	// Sega, missing SEGA
	mdFile := make([]byte, 0x10000)
	smdFile := make([]byte, 0x10000)
	mgdFile := make([]byte, 0x10000)
	for i := range mdFile {
		b := i / 0x4000
		h := (i - b*0x4000) / 0x2000
		h++
		b++
		mdFile[i] = byte((i % 2) * b)
		smdFile[i] = byte((h % 2) * b)
		if i >= 0x8000 {
			mgdFile[i] = 0
		} else {
			mgdFile[i] = byte(h + ((b - 1) * 2))
		}
	}
	mdPath := filepath.Join(dir, "nosega.md")
	smdPath := filepath.Join(dir, "nosega.smd")
	mgdPath := filepath.Join(dir, "nosega.mgd")
	err = ioutil.WriteFile(mdPath, mdFile, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: mdPath, SHA1: "526289b04144ebb25afcbeaf0febb1c0cd60bf79"})
	err = ioutil.WriteFile(smdPath, append(make([]byte, 512), smdFile...), 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: smdPath, SHA1: "526289b04144ebb25afcbeaf0febb1c0cd60bf79"})
	err = ioutil.WriteFile(mgdPath, mgdFile, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: mgdPath, SHA1: "526289b04144ebb25afcbeaf0febb1c0cd60bf79"})

	copy(mdFile[256:273], []byte("SEGA GENESIS    "))
	copy(smdFile[128:136], []byte("EAGNSS  "))
	copy(mgdFile[128:136], []byte("EAGNSS  "))
	copy(smdFile[8320:8328], []byte("SG EEI  "))
	copy(mgdFile[32896:32904], []byte("SG EEI  "))
	mdPath = filepath.Join(dir, "sega-md.md")
	smdPath = filepath.Join(dir, "sega-smd.md")
	mgdPath = filepath.Join(dir, "sega-mgd.md")
	err = ioutil.WriteFile(mdPath, mdFile, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: mdPath, SHA1: "5d2fa3c5c334d6f5c1c0959b040d4452b983d60f"})
	err = ioutil.WriteFile(smdPath, append(make([]byte, 512), smdFile...), 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: smdPath, SHA1: "5d2fa3c5c334d6f5c1c0959b040d4452b983d60f"})
	err = ioutil.WriteFile(mgdPath, mgdFile, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: mgdPath, SHA1: "5d2fa3c5c334d6f5c1c0959b040d4452b983d60f"})

	// NES
	nesHeaderv1 := []byte{0x4E, 0x45, 0x53, 0x1A, 0x02, 0x06, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	nesFilev1 := make([]byte, 16+32768+49152)
	nesHeaderv1Trainer := []byte{0x4E, 0x45, 0x53, 0x1A, 0x02, 0x06, 0x04, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	nesFilev1Trainer := make([]byte, 16+512+32768+49152)
	nesHeaderv2 := []byte{0x4E, 0x45, 0x53, 0x1A, 0x02, 0x06, 0, 0x08, 0, 0x11, 0, 0, 0, 0, 0, 0}
	nesFilev2 := make([]byte, 16+4227072+2146304)
	copy(nesFilev1, nesHeaderv1)
	copy(nesFilev1Trainer, nesHeaderv1Trainer)
	copy(nesFilev2, nesHeaderv2)
	for i := 16; i < len(nesFilev1); i++ {
		if i < 32768+16 {
			nesFilev1[i] = 0
			nesFilev1Trainer[i+512] = 0
		} else {
			nesFilev1[i] = 1
			nesFilev1Trainer[i+512] = 1
		}
	}
	for i := 16; i < len(nesFilev2); i++ {
		if i < 4227072+16 {
			nesFilev2[i] = 0
		} else {
			nesFilev2[i] = 1
		}
	}
	nesPathv1 := filepath.Join(dir, "nes-v1.nes")
	nesPathv1Trainer := filepath.Join(dir, "nes-v1-trainer.nes")
	nesPathv2 := filepath.Join(dir, "nes-v2.nes")
	err = ioutil.WriteFile(nesPathv1, nesFilev1, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: nesPathv1, SHA1: "310127efa1522ee9cb559ec502c0f6bb7fde308c"})
	err = ioutil.WriteFile(nesPathv1Trainer, nesFilev1Trainer, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: nesPathv1Trainer, SHA1: "310127efa1522ee9cb559ec502c0f6bb7fde308c"})
	err = ioutil.WriteFile(nesPathv2, nesFilev2, 0777)
	if err != nil {
		return data, err
	}
	data.Files = append(data.Files, File{Path: nesPathv2, SHA1: "60afb4f8dcb8e1d1c3ba48a8a836f80a89301f65"})

	// ZIP
	for _, f := range data.Files {
		p := fmt.Sprintf("%s.zip", f.Path)
		var w *os.File
		w, err = os.Create(p)
		if err != nil {
			return data, err
		}
		zw := zip.NewWriter(w)
		var zf io.Writer
		zf, err = zw.Create(filepath.Base(f.Path))
		if err != nil {
			return data, err
		}
		var fileData []byte
		fileData, err = ioutil.ReadFile(f.Path)
		if err != nil {
			return data, err
		}
		_, err = zf.Write(fileData)
		if err != nil {
			return data, err
		}
		err = zw.Close()
		if err != nil {
			return data, err
		}
		err = w.Close()
		if err != nil {
			return data, err
		}
		data.Files = append(data.Files, File{Path: p, SHA1: f.SHA1})
	}

	// GZIP
	for _, f := range data.Files {
		if filepath.Ext(f.Path) == ".zip" {
			continue
		}
		p := fmt.Sprintf("%s.gz", f.Path)
		var w *os.File
		w, err = os.Create(p)
		if err != nil {
			return data, err
		}
		zw := gzip.NewWriter(w)
		zw.Header.Name = filepath.Base(f.Path)
		var fileData []byte
		fileData, err = ioutil.ReadFile(f.Path)
		if err != nil {
			return data, err
		}
		_, err = zw.Write(fileData)
		if err != nil {
			return data, err
		}
		err = zw.Close()
		if err != nil {
			return data, err
		}
		data.Files = append(data.Files, File{Path: p, SHA1: f.SHA1})
	}
	return data, nil
}

type File struct {
	Path string
	SHA1 string
}
