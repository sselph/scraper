scraper
=======

An auto-scraper for EmulationStation written in Go using hashes.
This currently works with NES, SNES, N64, GB, GBC, GBA, MD, SMS, 32X, GG, PCE, A2600, LNX, MAME(see below) ROMs.

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
$ scraper
```

ROMs will be scanned and a gamelist.xml file will be created. All images will be placed inside the images folder.

MAME
----
The scraper now supports MAME but using file names instead of hashing. Since it uses a different DB and lookup method, several of the command line flags no longer apply. When the -mame flag is used it disables all other databases and the mamedb only has the one size of image so the flags about thumbnails, gdb, ovgdb, etc don't do anything.

```bash
$ scraper -mame
```

Command Line Flags
------------------
There are several command flags you can pass. To see a full list use -help

```bash
$ scraper -help
```

Raspberry Pi
------------
At the time of writing this raspbian has an old version of go 1.0.2 so you can cross-compile on another system or download an unofficial go binary from [http://dave.cheney.net/unofficial-arm-tarballs](http://dave.cheney.net/unofficial-arm-tarballs).

Build:

```bash
$ GOARM=6 GOARCH=arm GOOS=linux go build github.com/sselph/scraper
```

Usage:
Add thumb_only can speed things up since the pi doesn't have a ton of memory.

```bash
$ scraper -thumb_only
```

Used libraries
==============

| Package | Description | License |
| --- | --- | --- |
| [github.com/nfnt/resize](https://github.com/nfnt/resize) | resizes images | ISC |
| [github.com/kr/fs](https://github.com/kr/fs) | provides filesystem-related functions | New BSD |
| [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | provides sqlite | MIT |
| [github.com/mitchellh/go-homedir](https://github.com/mitchellh/go-homedir) | get user homedir | MIT |
| [github.com/syndtr/goleveldb](https://github.com/syndtr/goleveldb) | provides leveldb | Simplified BSD |
