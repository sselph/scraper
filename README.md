scraper
=======

A scraper for EmulationStation written in Go using hashes.
This currently only works with NES and SNES ROMs.

Installation
------------

```bash
$ go get github.com/sselph/scraper
$ go build github.com/sselph/scraper
```

Usage
-----

```bash
$ cd <rom directory>
$ mkdir images
$ scraper
```

ROMs will be scanned and a gamelist.xml file will be created. All images will be placed inside the images folder.
