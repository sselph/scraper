package main

import (
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/sselph/scraper/ds"
	"github.com/sselph/scraper/gdb"
	"github.com/sselph/scraper/rom"
	"github.com/sselph/scraper/ss"

	rh "github.com/sselph/scraper/rom/hash"
)

var hashFile = flag.String("hash_file", "", "The `file` containing hash information.")
var romDir = flag.String("rom_dir", ".", "The `directory` containing the roms file to process.")
var outputFile = flag.String("output_file", "gamelist.xml", "The XML `file` to output to.")
var imageDir = flag.String("image_dir", "images", "The `directory` to place downloaded images to locally.")
var imagePath = flag.String("image_path", "images", "The `path` to use for images in gamelist.xml.")
var imageSuffix = flag.String("image_suffix", "-image", "The `suffix` added after rom name when creating image files.")
var thumbSuffix = flag.String("thumb_suffix", "-thumb", "The `suffix` added after rom name when creating thumb files.")
var romPath = flag.String("rom_path", ".", "The `path` to use for roms in gamelist.xml.")
var maxWidth = flag.Uint("max_width", 400, "The max `width` of images. Larger images will be resized.")
var maxHeight = flag.Uint("max_height", 0, "The max `height` of images. Larger images will be resized.")
var workers = flag.Int("workers", 1, "Use `N` worker threads to process roms.")
var imgWorkers = flag.Int("img_workers", 0, "Use `N` worker threads to process images. If 0, then use the same value as workers.")
var retries = flag.Int("retries", 2, "Retry a rom `N` times on an error.")
var thumbOnly = flag.Bool("thumb_only", false, "Download the thumbnail for both the image and thumb (faster).")
var noThumb = flag.Bool("no_thumb", false, "Don't add thumbnails to the gamelist.")
var skipCheck = flag.Bool("skip_check", false, "Skip the check if thegamesdb.net is up.")
var nestedImageDir = flag.Bool("nested_img_dir", false, "Use a nested img directory structure that matches rom structure.")
var region = flag.String("region", "us,wor,eu,jp,fr,xx", "The order to choose for region if there is more than one for a value. xx is a special region that will choose any region.")
var lang = flag.String("lang", "en", "The order to choose for language if there is more than one for a value. (en, fr, es, de, pt)")
var startPprof = flag.Bool("start_pprof", false, "If true, start the pprof service used to profile the application.")
var useFilename = flag.Bool("use_filename", false, "If true, use the filename minus the extension as the game title in xml.")
var addNotFound = flag.Bool("add_not_found", false, "If true, add roms that are not found as an empty gamelist entry.")
var useNoIntroName = flag.Bool("use_nointro_name", true, "Use the name in the No-Intro DB instead of the one in the GDB.")
var mame = flag.Bool("mame", false, "If true we want to run in MAME mode.")
var mameImg = flag.String("mame_img", "t,m,s,c", "Comma separated order to prefer images, s=snap, t=title, m=marquee, c=cabniet, b=boxart, 3b=3D-boxart, fly=flyer.")
var mameSrcs = flag.String("mame_src", "adb,gdb", "Comma seperated order to prefer mame sources, ss=screenscraper, adb=arcadeitalia, mamedb=mamedb-mirror, gdb=theGamesDB-neogeo")
var consoleSrcs = flag.String("console_src", "gdb", "Comma seperated order to prefer console sources, ss=screenscraper, ovgdb=OpenVGDB, gdb=theGamesDB")
var stripUnicode = flag.Bool("strip_unicode", false, "If true, remove all non-ascii characters.")
var downloadImages = flag.Bool("download_images", true, "If false, don't download any images, instead see if the expected file is stored locally already.")
var scrapeAll = flag.Bool("scrape_all", false, "If true, scrape all systems listed in es_systems.cfg. All dir/path flags will be ignored.")
var consoleImg = flag.String("console_img", "b", "Comma seperated order to prefer images, s=snapshot, b=boxart, f=fanart, a=banner, l=logo, 3b=3D boxart, mix3=Standard 3 mix, mix4=Standard 4 mix.")
var imgFormat = flag.String("img_format", "jpg", "`jpg or png`, the format to write the images.")
var appendOut = flag.Bool("append", false, "If the gamelist file already exist skip files that are already listed and only append new files.")
var version = flag.Bool("version", false, "Print the release version and exit.")
var refreshOut = flag.Bool("refresh", false, "Information will be attempted to be downloaded again but won't remove roms that are not scraped.")
var extraExt = flag.String("extra_ext", "", "Comma separated list of extensions to also include in the scraper.")
var missing = flag.String("missing", "", "The `file` where information about ROMs that weren't scraped is added.")
var overviewLen = flag.Int("overview_len", 0, "If set it will truncate the overview of roms to `N` characters + ellipsis.")
var ssUser = flag.String("ss_user", "", "The `username` for registered ScreenScraper users.")
var ssPassword = flag.String("ss_password", "", "The `password` for registered ScreenScraper users.")
var updateCache = flag.Bool("update_cache", true, "If false, don't check for updates on locally cached files.")

var errUserCanceled = errors.New("user canceled")

var versionStr string

// exists checks if a file exists and contains data.
func exists(s string) bool {
	fi, err := os.Stat(s)
	return !os.IsNotExist(err) && fi.Size() > 0
}

func dirExists(s string) bool {
	fi, err := os.Stat(s)
	return !os.IsNotExist(err) && fi.IsDir()
}

func isHidden(f string) bool {
	b := filepath.Base(f)
	return b != "." && strings.HasPrefix(b, ".")
}

type result struct {
	ROM *rom.ROM
	XML *rom.GameXML
	Err error
}

func done(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// worker is a function to process roms from a channel.
func worker(ctx context.Context, sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts, results chan result, roms chan *rom.ROM, wg *sync.WaitGroup) {
	defer wg.Done()
	for r := range roms {
		if done(ctx) {
			break
		}
		res := result{ROM: r}
		for try := 0; try <= *retries; try++ {
			if done(ctx) {
				break
			}
			log.Printf("INFO: Starting: %s", r.Path)
			if err := r.GetGame(ctx, sources, gameOpts); err != nil {
				log.Printf("ERR: error processing %s: %s", r.Path, err)
				res.Err = err
				if err == ds.ErrNotFound {
					break
				} else {
					continue
				}
			}
			if r.NotFound {
				log.Printf("INFO: %s, %s", r.Path, ds.ErrNotFound)
			}
			xml, err := r.XML(ctx, xmlOpts)
			if err != nil {
				log.Printf("ERR: error processing %s: %s", r.Path, err)
				res.Err = err
				continue
			}
			res.XML = xml
			break
		}
		results <- res
	}
}

// crawlROMs crawls the rom directory and processes the files.
func crawlROMs(ctx context.Context, gl *rom.GameListXML, sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts) error {
	var missingCSV *csv.Writer
	var gdbDS *ds.GDB
	if *missing != "" {
		f, err := os.Create(*missing)
		if err != nil {
			return err
		}
		missingCSV = csv.NewWriter(f)
		defer func() {
			missingCSV.Flush()
			if err := missingCSV.Error(); err != nil {
				log.Fatal(err)
			}
			f.Close()
		}()
		if err := missingCSV.Write([]string{"Game", "Error", "Hash", "Extra"}); err != nil {
			return err
		}
		for _, d := range sources {
			switch d := d.(type) {
			case *ds.GDB:
				gdbDS = d
			}
		}
	}

	existing := make(map[string]bool)

	if !dirExists(xmlOpts.RomDir) {
		log.Printf("ERR %s: does not exists", xmlOpts.RomDir)
		return nil
	}

	if *extraExt != "" {
		extraSlice := strings.Split(*extraExt, ",")
		for _, e := range extraSlice {
			if e[0] != '.' {
				rh.AddExtra("." + e)
			} else {
				rh.AddExtra(e)
			}
		}
	}

	var filterGL []*rom.GameXML
	for _, x := range gl.GameList {
		p, err := filepath.Rel(xmlOpts.RomXMLDir, x.Path)
		if err != nil {
			log.Printf("Can't find original path: %s", x.Path)
		}
		f := filepath.Join(xmlOpts.RomDir, p)
		if exists(f) {
			filterGL = append(filterGL, x)
		}
		switch {
		case *appendOut:
			existing[f] = true
		case *refreshOut:
			existing[x.Path] = true
		}
	}
	gl.GameList = filterGL

	var wg sync.WaitGroup
	results := make(chan result, *workers)
	roms := make(chan *rom.ROM, 2**workers)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(ctx, sources, xmlOpts, gameOpts, results, roms, &wg)
	}
	go func() {
		defer wg.Done()
		for r := range results {
			if r.XML == nil {
				if *missing == "" {
					continue
				}
				files := []string{r.ROM.Path}
				if r.ROM.Cue {
					files = append(files, r.ROM.Bins...)
				}
				for _, file := range files {
					var hash, extra string
					if gdbDS != nil {
						var err error
						hash, err = gdbDS.Hash(file)
						if err != nil {
							log.Printf("ERR: Can't hash file %s", file)
						}
						name := gdbDS.GetName(file)
						if name != "" && r.Err == ds.ErrNotFound {
							extra = "hash found but no GDB ID"
						}
					}
					if err := missingCSV.Write([]string{file, r.Err.Error(), hash, extra}); err != nil {
						log.Printf("ERR: Can't write to %s", *missing)
					}
				}
				continue
			}
			if r.XML.Image == "" && *missing != "" {
				var hash string
				if gdbDS != nil {
					var err error
					hash, err = gdbDS.Hash(r.ROM.Path)
					if err != nil {
						log.Printf("ERR: Can't hash file %s", r.ROM.Path)
					}
				}
				if err := missingCSV.Write([]string{r.ROM.FileName, "", hash, "missing image"}); err != nil {
					log.Printf("ERR: Can't write to %s", *missing)
				}
			}
			if existing[r.XML.Path] && *refreshOut {
				for i, g := range gl.GameList {
					if g.Path != r.XML.Path {
						continue
					}
					r.XML.Favorite = g.Favorite
					r.XML.LastPlayed = g.LastPlayed
					r.XML.PlayCount = g.PlayCount
					copy(gl.GameList[i:], gl.GameList[i+1:])
					gl.GameList = gl.GameList[:len(gl.GameList)-1]
				}
			}
			gl.Append(r.XML)
		}
	}()
	bins := make(map[string]bool)
	if !*mame {
		err := filepath.Walk(xmlOpts.RomDir, func(f string, fi os.FileInfo, err error) error {
			if done(ctx) {
				return ctx.Err()
			}
			if err != nil {
				log.Printf("ERR: Processing: %s, %s", f, err)
				return nil
			}
			if isHidden(f) {
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if fi.IsDir() {
				return nil
			}
			r, err := rom.NewROM(f)
			if err != nil {
				log.Printf("ERR: Processing: %s, %s", f, err)
				return nil
			}
			if !r.Cue {
				return nil
			}
			for _, b := range r.Bins {
				bins[b] = true
			}
			bins[f] = true
			if existing[f] && !*refreshOut {
				log.Printf("INFO: Skipping %s, already in gamelist.", f)
				return nil
			}
			roms <- r
			return nil
		})
		if err != nil && err != context.Canceled {
			return err
		}
	}
	err := filepath.Walk(xmlOpts.RomDir, func(f string, fi os.FileInfo, err error) error {
		if done(ctx) {
			return ctx.Err()
		}
		if err != nil {
			log.Printf("ERR: Processing: %s, %s", f, err)
			return nil
		}
		if isHidden(f) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if filepath.Ext(f) == ".daphne" {
			return filepath.SkipDir
		}
		if existing[f] && !*refreshOut {
			log.Printf("INFO: Skipping %s, already in gamelist.", f)
			return nil
		}
		r, err := rom.NewROM(f)
		if err != nil {
			log.Printf("ERR: Processing: %s, %s", f, err)
			return nil
		}
		if *mame {
			if r.Ext == ".zip" || r.Ext == ".7z" || rh.HasExtra(r.Ext) {
				roms <- r
			}
			return nil
		}
		if !bins[f] && (rh.KnownExt(r.Ext) || r.Ext == ".svm" || r.Ext == ".daphne" || r.Ext == ".7z") {
			roms <- r
		}
		return nil
	})
	if err != nil && err != context.Canceled {
		return err
	}
	close(roms)
	wg.Wait()
	wg.Add(1)
	close(results)
	wg.Wait()
	if done(ctx) {
		return errUserCanceled
	}
	return nil
}

func mkDir(d string) error {
	fi, err := os.Stat(d)
	switch {
	case os.IsNotExist(err):
		return os.MkdirAll(d, 0775)
	case err != nil:
		return err
	case fi.IsDir():
		return nil
	}
	return fmt.Errorf("%s is a file not a directory", d)
}

// scrape handles scraping and wriiting the XML.
func scrape(ctx context.Context, sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts) error {
	var err error
	xmlOpts.RomDir, err = filepath.EvalSymlinks(xmlOpts.RomDir)
	if err != nil {
		return err
	}
	gl := &rom.GameListXML{}
	if *appendOut || *refreshOut {
		f, err := os.Open(*outputFile)
		if err != nil {
			log.Printf("ERR: Can't open %s, creating new file. error %q", *outputFile, err)
		} else {
			decoder := xml.NewDecoder(f)
			if err := decoder.Decode(gl); err != nil {
				log.Printf("ERR: Can't open %s, creating new file. error %q", *outputFile, err)
			}
			f.Close()
		}
	}
	cerr := crawlROMs(ctx, gl, sources, xmlOpts, gameOpts)
	if cerr != nil && cerr != errUserCanceled {
		return cerr
	}
	output, err := xml.MarshalIndent(gl, "  ", "    ")
	if err != nil {
		return err
	}
	if len(gl.GameList) == 0 {
		return cerr
	}
	err = mkDir(filepath.Dir(*outputFile))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(*outputFile, append([]byte(xml.Header), output...), 0664)
	if err != nil {
		return err
	}
	return cerr
}

// System represents a single system in es_systems.cfg
type System struct {
	Name      string `xml:"name"`
	Path      string `xml:"path"`
	Extension string `xml:"extension"`
	Platform  string `xml:"platform"`
}

// ESSystems represents es_systems.cfg
type ESSystems struct {
	XMLName xml.Name `xml:"systemList"`
	Systems []System `xml:"system"`
}

// getSystems finds and parses es_systems.cfg to get rom folders.
func getSystems() ([]System, error) {
	hd, err := homedir.Dir()
	if err != nil {
		return nil, err
	}
	p := filepath.Join(hd, ".emulationstation", "es_systems.cfg")
	ap := "/etc/emulationstation/es_systems.cfg"
	if !exists(p) && !exists(ap) {
		return nil, fmt.Errorf("%s and %s not found", p, ap)
	}
	if exists(ap) && !exists(p) {
		p = ap
	}
	d, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	v := &ESSystems{}
	err = xml.Unmarshal(d, &v)
	if err != nil {
		return nil, err
	}
	var out []System
	prefix := string([]rune{'~', filepath.Separator})
	for _, s := range v.Systems {
		if len(s.Path) >= 2 && s.Path[:2] == prefix {
			s.Path = filepath.Join(hd, s.Path[2:])
		}
		out = append(out, s)
	}
	return out, nil
}

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	defer signal.Stop(stopChan)
	go func() {
		<-stopChan
		cancel()
	}()

	if *version {
		fmt.Println(versionStr)
		return
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	if *startPprof {
		go http.ListenAndServe(":8080", nil)
	}
	if *imgWorkers == 0 {
		*imgWorkers = *workers
	}
	rom.SetMaxImg(*imgWorkers)

	xmlOpts := &rom.XMLOpts{
		RomDir:     *romDir,
		RomXMLDir:  *romPath,
		NestImgDir: *nestedImageDir,
		ImgDir:     *imageDir,
		ImgXMLDir:  *imagePath,
		ImgSuffix:  *imageSuffix,
		ThumbOnly:  *thumbOnly,
		NoDownload: !*downloadImages,
		ImgFormat:  *imgFormat,
		ImgWidth:   *maxWidth,
		ImgHeight:  *maxHeight,
	}
	var aImg []ds.ImgType
	var cImg []ds.ImgType
	var ssRegions []string
	var ssLangs []string
	for _, t := range strings.Split(*mameImg, ",") {
		aImg = append(aImg, ds.ImgType(t))
	}
	for _, t := range strings.Split(*consoleImg, ",") {
		cImg = append(cImg, ds.ImgType(t))
	}
	for _, r := range strings.Split(*region, ",") {
		if r != "" {
			ssRegions = append(ssRegions, r)
		}
	}
	for _, l := range strings.Split(*lang, ",") {
		if l != "" {
			ssLangs = append(ssLangs, l)
		}
	}
	gameOpts := &rom.GameOpts{
		AddNotFound:    *addNotFound,
		NoPrettyName:   !*useNoIntroName,
		UseFilename:    *useFilename,
		NoStripUnicode: !*stripUnicode,
		OverviewLen:    *overviewLen,
	}

	var arcadeSources []ds.DS
	var consoleSources []ds.DS

	if !*scrapeAll {
		if *mame {
			*consoleSrcs = ""
		} else {
			*mameSrcs = ""
		}
	}

	log.Printf("arcade  srcs: %v", *mameSrcs)
	log.Printf("console srcs: %v", *consoleSrcs)

	var needHM, needHasher bool
	cSrcNames := strings.Split(*consoleSrcs, ",")
	aSrcNames := strings.Split(*mameSrcs, ",")
	for _, s := range cSrcNames {
		switch s {
		case "gdb", "ss":
			needHM = true
			needHasher = true
		case "ovgdb":
			needHasher = true
		}
	}
	for _, s := range aSrcNames {
		switch s {
		case "gdb":
			needHM = true
		}
	}

	var hasher *ds.Hasher
	var err error
	if needHasher {
		hasher, err = ds.NewHasher(sha1.New, *workers)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	var hm *ds.HashMap
	if needHM {
		if *hashFile != "" {
			hm, err = ds.FileHashMap(*hashFile)
		} else {
			hm, err = ds.CachedHashMap(ctx, "", *updateCache)
		}
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	dev, err := ss.DeobfuscateDevInfo()
	if err != nil {
		fmt.Printf("Can't access SS dev information: %q", err)
		return
	}

	for _, src := range cSrcNames {
		switch src {
		case "":
		case "gdb":
			if !*skipCheck {
				if ok := gdb.IsUp(ctx); !ok {
					fmt.Println("It appears that thegamesdb.net isn't up. If you are sure it is use -skip_check to bypass this error.")
					continue
				}
			}
			consoleSources = append(consoleSources, &ds.GDB{HM: hm, Hasher: hasher})
			consoleSources = append(consoleSources, &ds.ScummVM{HM: hm})
			consoleSources = append(consoleSources, &ds.Daphne{HM: hm})
			consoleSources = append(consoleSources, &ds.NeoGeo{HM: hm})
		case "ss":
			t := ss.Threads(ctx, dev, ss.UserInfo{*ssUser, *ssPassword})
			limit := make(chan struct{}, t)
			for i := 0; i < t; i++ {
				limit <- struct{}{}
			}
			ssDS := &ds.SS{
				HM:     hm,
				Hasher: hasher,
				Dev:    dev,
				User:   ss.UserInfo{*ssUser, *ssPassword},
				Width:  int(*maxWidth),
				Height: int(*maxHeight),
				Region: ssRegions,
				Lang:   ssLangs,
				Limit:  limit,
			}
			consoleSources = append(consoleSources, ssDS)
		case "ovgdb":
			o, err := ds.NewOVGDB(ctx, hasher, *updateCache)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer o.Close()
			consoleSources = append(consoleSources, o)
		default:
			fmt.Printf("unknown console source :%q", src)
			return
		}
	}
	for _, src := range aSrcNames {
		switch src {
		case "":
		case "ss":
			t := ss.Threads(ctx, dev, ss.UserInfo{*ssUser, *ssPassword})
			limit := make(chan struct{}, t)
			for i := 0; i < t; i++ {
				limit <- struct{}{}
			}
			ssMDS := &ds.SSMAME{
				Dev:    dev,
				User:   ss.UserInfo{*ssUser, *ssPassword},
				Width:  int(*maxWidth),
				Height: int(*maxHeight),
				Region: ssRegions,
				Lang:   ssLangs,
				Limit:  limit,
			}
			arcadeSources = append(arcadeSources, ssMDS)
		case "mamedb":
			mds, err := ds.NewMAME(ctx, "", *updateCache)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer mds.Close()
			arcadeSources = append(arcadeSources, mds)
		case "gdb":
			arcadeSources = append(arcadeSources, &ds.NeoGeo{HM: hm})
		case "adb":
			arcadeSources = append(arcadeSources, &ds.ADB{})
		default:
			fmt.Printf("Invalid MAME source %q\n", src)
			return
		}
	}

	if !*scrapeAll {
		var sources []ds.DS
		if *mame {
			sources = arcadeSources
			xmlOpts.ImgPriority = aImg
		} else {
			sources = consoleSources
			xmlOpts.ImgPriority = cImg
		}
		err := scrape(ctx, sources, xmlOpts, gameOpts)
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		systems, err := getSystems()
		if err != nil {
			fmt.Println(err)
			return
		}
		origMissing := *missing
		for _, s := range systems {
			log.Printf("Starting System %s", s.Path)
			xmlOpts.RomDir = s.Path
			xmlOpts.RomXMLDir = s.Path
			p := filepath.Join(s.Path, "images")
			xmlOpts.ImgDir = p
			xmlOpts.ImgXMLDir = p
			out := filepath.Join(s.Path, "gamelist.xml")
			*outputFile = out
			if origMissing != "" {
				*missing = fmt.Sprintf("%s_%s", s.Name, origMissing)
			}
			var sources []ds.DS
			switch s.Platform {
			case "arcade", "neogeo":
				sources = arcadeSources
				xmlOpts.ImgPriority = aImg
			default:
				sources = consoleSources
				xmlOpts.ImgPriority = cImg
			}
			err := scrape(ctx, sources, xmlOpts, gameOpts)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
}
