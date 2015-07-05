rename
=======

This command renames roms based on their hash and no-intro name.

How it Works
------------
The script takes each file, hashes it, looks in its hash DB and if there is a no-intro name, renames the file.

Installation
------------

Make sure you have go version 1.2 or later installed.

```bash
$ go version
go version go1.2.1 linux/amd64
```

Fetch and build.

```bash
$ go get github.com/sselph/scraper/rename
$ go build github.com/sselph/scraper/rename
```

Usage
-----

Dry-Run
```bash
$ cd <rom directory>
$ rename *.nes
```

Actually rename
```bash
$ cd <rom directory>
$ rename -dry_run=false *.nes
```
