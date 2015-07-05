package main

import (
	"crypto/sha1"
	"flag"
	"fmt"
	"github.com/sselph/scraper/ds"
	rh "github.com/sselph/scraper/rom/hash"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var romDir = flag.String("rom_dir", ".", "The directory containing the roms file to process.")
var dryRun = flag.Bool("dry_run", true, "Print what will renamed.")

// exists checks if a file exists and contains data.
func exists(s string) bool {
	fi, err := os.Stat(s)
	return !os.IsNotExist(err) && fi.Size() > 0
}

func rename(source *ds.GDB, files []string) error {
	for _, p := range files {
		dir := filepath.Dir(p)
		ext := filepath.Ext(p)
		n := source.GetName(p)
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			continue
		}
		if !rh.KnownExt(ext) {
			continue
		}
		if n == "" {
			continue
		}
		newPath := filepath.Join(dir, n+ext)
		if newPath == p {
			continue
		}
		log.Printf("%s -> %s", p, newPath)
		if *dryRun {
			continue
		}
		err = os.Rename(p, newPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	hasher, err := ds.NewHasher(sha1.New)
	if err != nil {
		fmt.Println(err)
		return
	}
	hm, err := ds.CachedHashMap("")
	if err != nil {
		fmt.Println(err)
		return
	}
	gdb := &ds.GDB{HM: hm, Hasher: hasher}
	err = rename(gdb, flag.Args()[1:])
	if err != nil {
		fmt.Println(err)
		return
	}
}
