scraper
=======

An auto-scraper for EmulationStation written in Go using hashes.
This currently works with NES, SNES, GB, GBC, GBA, MD, SMS, 32X, GG, PCE, A2600 ROMs.

How it Works
------------
The script works by crawling a directory of ROM files looking for known extensions. When it finds a file it hashes the ROM data minus any headers or special file formatting with the goal of hashing only the data pulled from the original game. It compares this hash to a DB I've compiled to look up the correct game in theGamesDB.net. It downloads the metadata and builds the gamelist.xml file.

Installation
------------

Make sure you have go version 1.2 or later installed.

```bash
$ go version
go version go1.2.1 linux/amd64
```

Fetch and build.

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

Raspberry Pi
------------
The Raspberry Pi's build process is a little different and I recommend building and running on a machine other than the PI. At the time of writing this raspbian has an old version of go 1.0.2 so it can't handle progressive scan jpeg files also when building for the pi you have to specificy the type of ARM processor.

Build:

```bash
$ GOARM=5 go build github.com/sselph/scraper
```

Usage:
Add thumb_only so that it doesn't download the larger progressive jpeg files.

```bash
$ scraper -thumb_only
```

Used libraries
==============

| Package | Description | License |
| --- | --- | --- |
| [github.com/nfnt/resize](https://github.com/nfnt/resize) | resizes images | ISC |
| [github.com/kr/fs](https://github.com/kr/fs) | provides filesystem-related functions | New BSD |
