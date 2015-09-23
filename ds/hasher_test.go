package ds

import (
	"crypto/sha1"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type File struct {
	Path string
	SHA1 string
}

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
	binHash := "2c2ceccb5ec5574f791d45b63c940cff20550f9a"
	p := filepath.Join(dir, "test.bin")
	f, err := os.Create(p)
	if err != nil {
		return data, err
	}
	for i := 0; i < 100; i++ {
		_, err = f.Write(make([]byte, 1024*1024))
		if err != nil {
			return data, err
		}
	}
	data.Files = append(data.Files, File{Path: p, SHA1: binHash})
	return data, nil
}

func TestHasher(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Error(err)
	}
	defer d.Close()
	d.Files = append(d.Files, File{Path: "doesnotexist"})
	h, err := NewHasher(sha1.New)
	if err != nil {
		t.Error(err)
	}
	var wg sync.WaitGroup
	f := func(f File) {
		defer wg.Done()
		s, err := h.Hash(f.Path)
		if f.SHA1 != "" && s != f.SHA1 {
			t.Errorf("h.Hash(%q) => %q; want %q", f.Path, s, f.SHA1)
		} else if f.SHA1 == "" && err == nil {
			t.Errorf("h.Hash(%q) => err = nil; want !nil", f.Path)
		}
	}
	wg.Add(10)
	go f(d.Files[0])
	go f(d.Files[0])
	go f(d.Files[0])
	go f(d.Files[0])
	go f(d.Files[0])
	go f(d.Files[1])
	go f(d.Files[1])
	go f(d.Files[1])
	go f(d.Files[1])
	go f(d.Files[1])
	wg.Wait()
	wg.Add(2)
	f(d.Files[0])
	f(d.Files[1])
}
