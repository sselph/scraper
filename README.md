scraper
=======
[![Build Status](https://travis-ci.org/sselph/scraper.svg?branch=master)](https://travis-ci.org/sselph/scraper) [![GoDoc](https://godoc.org/github.com/sselph/scraper?status.svg)](https://godoc.org/github.com/sselph/scraper)

An auto-scraper for EmulationStation written in Go using hashes.
This currently works with NES, SNES, N64, GB, GBC, GBA, MD, SMS, 32X, GG, PCE, A2600, LNX, MAME/FBA(see below), Dreamcast(bin/gdi), PSX(bin/cue), ScummVM, SegaCD, WonderSwan, WonderSwan Color ROMs.

How it Works
------------
The script works by crawling a directory of ROM files looking for known extensions. When it finds a file it hashes the ROM data minus any headers or special file formatting with the goal of hashing only the data pulled from the original game. It compares this hash to a DB from [OpenVGDB](https://github.com/OpenVGDB/OpenVGDB) to look up the correct game in theGamesDB.net. It downloads the metadata and builds the gamelist.xml file.

If you have RetroPie-Setup 3.1 or later you can follow [the instructions on their wiki](https://github.com/RetroPie/RetroPie-setup/wiki/scraper) instead.

Donate
------
I don't accept donations but I'm raising money for Children's Healthcare of Atlanta:

[![Extra Life](https://bfapps1.boundlessfundraising.com/badge/extralife/display/319247/539/status.jpg)](https://goo.gl/diu5oU)

Or feel free to donate to:

[retropie](https://retropie.org.uk/donate/)  
[ArcadeItalia](http://adb.arcadeitalia.net/)  
[ScreenScraper](https://screenscraper.fr/)  
[theGamesDB](http://thegamesdb.net/)

Installation
------------

Make sure you have go version 1.7 or later installed.

```bash
$ go version
go version go1.8 linux/amd64
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
$ scraper
```

ROMs will be scanned and a gamelist.xml file will be created. All images will be placed inside the images folder.

MAME/FBA
----
The scraper now supports MAME/FBA but using file names instead of hashing. Since it uses a different DB and lookup method, several of the command line flags no longer apply. When the -mame flag is used it disables all other databases and the mamedb only has the one size of image so the flags about thumbnails, gdb, ovgdb, etc don't do anything.

```bash
$ scraper -mame
```

You can choose your preference of image type with the mame_img flag. If you prefer marquees but want to fallback to titles then snapshots you can do:
```bash
$ scraper -mame -mame_img "m,t,s"
```

Command Line Flags
------------------
There are several command flags you can pass. To see a full list use -help

```bash
$ scraper -help
```

Raspberry Pi
------------

*Note:* Scraper is included in RetroPie-Setup from [version 3.1](https://github.com/RetroPie/RetroPie-Setup/releases/tag/3.1) and can be [accessed from their UI](https://github.com/RetroPie/RetroPie-setup/wiki/scraper). Otherwise you can use the instructions below.

At the time of writing this raspbian has an old version of go 1.3.3 so you can cross-compile on another system or download a [recent version of Go](https://golang.org/dl/)

### Build:

#### Rpi v1
```bash
$ GOARM=6 GOARCH=arm GOOS=linux go build github.com/sselph/scraper
```
#### Rpi v2
```bash
$ GOARM=7 GOARCH=arm GOOS=linux go build github.com/sselph/scraper
```

### Install from my Binaries

Replace the release_name with the release like v1.2.6

#### Rpi v1
```bash
$ wget https://github.com/sselph/scraper/releases/download/<release_name>/scraper_rpi.zip
$ sudo unzip scraper_rpi.zip scraper -d /usr/local/bin/
```
#### Rpi v2
```bash
$ wget https://github.com/sselph/scraper/releases/download/<release_name>/scraper_rpi2.zip
$ sudo unzip scraper_rpi2.zip scraper -d /usr/local/bin/
```

### Usage
Add thumb_only can speed things up since the pi doesn't have a ton of memory.

#### Single System
```bash
$ cd ~/RetroPie/roms/<rom_dir>
$ scraper -thumb_only
```

#### All Systems
```bash
$ scraper -scrape_all -thumb_only
```

Used libraries
==============

| Package | Description | License |
| --- | --- | --- |
| [github.com/nfnt/resize](https://github.com/nfnt/resize) | resizes images | ISC |
| [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | provides sqlite | MIT |
| [github.com/mitchellh/go-homedir](https://github.com/mitchellh/go-homedir) | get user homedir | MIT |
| [github.com/syndtr/goleveldb](https://github.com/syndtr/goleveldb) | provides leveldb | Simplified BSD |
| [github.com/hashicorp/golang-lru](https://github.com/hashicorp/golang-lru) | provides a LRU cache | Mozilla |
| [github.com/kjk/lzmadec](https://github.com/kjk/lzmadec) | provides access to 7z binary | MIT |
