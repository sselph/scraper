package main

import (
	"crypto/sha1"
	"fmt"
	"log"
	"os"

	"github.com/sselph/scraper/ds"
)

func main() {
	files := os.Args[1:]
	hasher, err := ds.NewHasher(sha1.New, 1)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			log.Fatal(err)
		}
		if fi.IsDir() {
			continue
		}
		h, err := hasher.Hash(file)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s  %s\n", h, file)
	}
}
