package main

/*
A sample program that extracts all files from a given .7z file
to current directory.
*/

import (
	"fmt"
	"os"

	"github.com/kjk/lzmadec"
)

func usageAndExit() {
	fmt.Printf("usage: test file.7z\n")
	os.Exit(1)
}

func main() {
	if len(os.Args) != 2 {
		usageAndExit()
	}
	path := os.Args[1]
	a, err := lzmadec.NewArchive(path)
	if err != nil {
		fmt.Printf("lzmadec.NewArchive('%s') failed with '%s'\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("opened archive '%s'\n", path)
	fmt.Printf("Extracting %d entries\n", len(a.Entries))
	for _, e := range a.Entries {
		err = a.ExtractToFile(e.Path, e.Path)
		if err != nil {
			fmt.Printf("a.ExtractToFile('%s') failed with '%s'\n", e.Path, err)
			os.Exit(1)
		}
		fmt.Printf("Extracted '%s'\n", e.Path)
	}
}
