package main

import (
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

	"github.com/kr/fs"
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
var workers = flag.Int("workers", 1, "Use `N` worker threads to process roms.")
var imgWorkers = flag.Int("img_workers", 0, "Use `N` worker threads to process images. If 0, then use the same value as workers.")
var retries = flag.Int("retries", 2, "Retry a rom `N` times on an error.")
var thumbOnly = flag.Bool("thumb_only", false, "Download the thumbnail for both the image and thumb (faster).")
var noThumb = flag.Bool("no_thumb", false, "Don't add thumbnails to the gamelist.")
var skipCheck = flag.Bool("skip_check", false, "Skip the check if thegamesdb.net is up.")
var nestedImageDir = flag.Bool("nested_img_dir", false, "Use a nested img directory structure that matches rom structure.")
var useGDB = flag.Bool("use_gdb", true, "Use the hash.csv and theGamesDB metadata.")
var useOVGDB = flag.Bool("use_ovgdb", false, "Use the OpenVGDB if the hash isn't in hash.csv.")
var useSS = flag.Bool("use_ss", false, "Use the ScreenScraper.fr as a datasource.")
var region = flag.String("region", "us,eu,jp,fr,xx", "The order to choose for region if there is more than one for a value. (us, eu, jp, fr, xx)")
var lang = flag.String("lang", "en", "The order to choose for language if there is more than one for a value. (en, fr, es, de, pt)")
var startPprof = flag.Bool("start_pprof", false, "If true, start the pprof service used to profile the application.")
var useFilename = flag.Bool("use_filename", false, "If true, use the filename minus the extension as the game title in xml.")
var addNotFound = flag.Bool("add_not_found", false, "If true, add roms that are not found as an empty gamelist entry.")
var useNoIntroName = flag.Bool("use_nointro_name", true, "Use the name in the No-Intro DB instead of the one in the GDB.")
var mame = flag.Bool("mame", false, "If true we want to run in MAME mode.")
var mameImg = flag.String("mame_img", "t,m,s,c", "Comma separated order to prefer images, s=snap, t=title, m=marquee, c=cabniet.")
var stripUnicode = flag.Bool("strip_unicode", true, "If true, remove all non-ascii characters.")
var downloadImages = flag.Bool("download_images", true, "If false, don't download any images, instead see if the expected file is stored locally already.")
var scrapeAll = flag.Bool("scrape_all", false, "If true, scrape all systems listed in es_systems.cfg. All dir/path flags will be ignored.")
var gdbImg = flag.String("gdb_img", "", "Deprecated, see console_img. This will be removed soon.")
var consoleImg = flag.String("console_img", "b", "Comma seperated order to prefer images, s=snapshot, b=boxart, f=fanart, a=banner, l=logo, 3b=3D boxart.")
var imgFormat = flag.String("img_format", "jpg", "`jpg or png`, the format to write the images.")
var appendOut = flag.Bool("append", false, "If the gamelist file already exist skip files that are already listed and only append new files.")
var version = flag.Bool("version", false, "Print the release version and exit.")
var refreshOut = flag.Bool("refresh", false, "Information will be attempted to be downloaded again but won't remove roms that are not scraped.")
var extraExt = flag.String("extra_ext", "", "Comma separated list of extensions to also include in the scraper.")
var missing = flag.String("missing", "", "The `file` where information about ROMs that weren't scraped is added.")
var overviewLen = flag.Int("overview_len", 0, "If set it will truncate the overview of roms to `N` characters + ellipsis.")

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

type result struct {
	ROM *rom.ROM
	XML *rom.GameXML
	Err error
}

// worker is a function to process roms from a channel.
func worker(sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts, results chan result, roms chan *rom.ROM, wg *sync.WaitGroup) {
	defer wg.Done()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	var stop bool
	go func() {
		<-sig
		stop = true
	}()
	for r := range roms {
		if stop {
			continue
		}
		res := result{ROM: r}
		for try := 0; try <= *retries; try++ {
			if stop {
				break
			}
			log.Printf("INFO: Starting: %s", r.Path)
			err := r.GetGame(sources, gameOpts)
			if err != nil {
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
			xml, err := r.XML(xmlOpts)
			if err != nil {
				log.Printf("ERR: error processing %s: %s", r.Path, err)
				continue
			}
			res.XML = xml
			break
		}
		results <- res
	}
}

// cancelTransport is a special HTTP transport that tracks pending requests so they can be cancelled.
type cancelTransport struct {
	mu      sync.Mutex
	Pending map[*http.Request]struct{}
	T       *http.Transport
	stop    bool
}

func (t *cancelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if t.stop {
		t.mu.Unlock()
		return nil, fmt.Errorf("Cancelled")
	}
	t.Pending[req] = struct{}{}
	t.mu.Unlock()
	resp, err := t.T.RoundTrip(req)
	t.mu.Lock()
	delete(t.Pending, req)
	if t.stop {
		t.mu.Unlock()
		return nil, fmt.Errorf("Cancelled")
	}
	t.mu.Unlock()
	return resp, err
}

func (t *cancelTransport) Stop() {
	t.mu.Lock()
	t.stop = true
	for req := range t.Pending {
		t.T.CancelRequest(req)
	}
	t.Pending = make(map[*http.Request]struct{})
	t.mu.Unlock()
}

// newCancelTransport wraps a transport to create a cancelTransport
func newCancelTransport(t *http.Transport) *cancelTransport {
	ct := &cancelTransport{T: t}
	ct.Pending = make(map[*http.Request]struct{})
	return ct
}

// crawlROMs crawls the rom directory and processes the files.
func crawlROMs(gl *rom.GameListXML, sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts) error {
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
	var ct http.RoundTripper = newCancelTransport(http.DefaultTransport.(*http.Transport))
	http.DefaultClient.Transport = ct

	existing := make(map[string]struct{})

	if !dirExists(xmlOpts.RomDir) {
		log.Printf("ERR %s: does not exists", xmlOpts.RomDir)
		return nil
	}

	extraMap := make(map[string]struct{})
	if *extraExt != "" {
		extraSlice := strings.Split(*extraExt, ",")
		for _, e := range extraSlice {
			if e[0] != '.' {
				extraMap["."+e] = struct{}{}
			} else {
				extraMap[e] = struct{}{}
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
			existing[f] = struct{}{}
		case *refreshOut:
			existing[x.Path] = struct{}{}
		}
	}
	gl.GameList = filterGL

	var wg sync.WaitGroup
	results := make(chan result, *workers)
	roms := make(chan *rom.ROM, 2**workers)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(sources, xmlOpts, gameOpts, results, roms, &wg)
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
			if _, ok := existing[r.XML.Path]; ok && *refreshOut {
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
	var stop bool
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	go func() {
		for {
			<-sig
			if !stop {
				stop = true
				log.Println("Stopping, ctrl-c again to stop now.")
				ct.(*cancelTransport).Stop()
				for _ = range roms {
				}
				continue
			}
			panic("AHHHH!")
		}
	}()
	bins := make(map[string]struct{})
	if !*mame {
		walker := fs.Walk(xmlOpts.RomDir)
		for walker.Step() {
			if stop {
				break
			}
			if err := walker.Err(); err != nil {
				return err
			}
			f := walker.Path()
			if b := filepath.Base(f); b != "." && strings.HasPrefix(b, ".") {
				walker.SkipDir()
				continue
			}
			r, err := rom.NewROM(f)
			if err != nil {
				log.Printf("ERR: Processing: %s, %s", f, err)
				continue
			}
			if !r.Cue {
				continue
			}
			for _, b := range r.Bins {
				bins[b] = struct{}{}
			}
			bins[f] = struct{}{}
			if _, ok := existing[f]; !*refreshOut && ok {
				log.Printf("INFO: Skipping %s, already in gamelist.", f)
				continue
			}
			roms <- r
		}
	}
	walker := fs.Walk(xmlOpts.RomDir)
	for walker.Step() {
		if stop {
			break
		}
		if err := walker.Err(); err != nil {
			return err
		}
		f := walker.Path()
		if b := filepath.Base(f); b != "." && strings.HasPrefix(b, ".") {
			walker.SkipDir()
			continue
		}
		if filepath.Ext(f) == ".daphne" {
			walker.SkipDir()
		}
		if _, ok := existing[f]; !*refreshOut && ok {
			log.Printf("INFO: Skipping %s, already in gamelist.", f)
			continue
		}
		r, err := rom.NewROM(f)
		if err != nil {
			log.Printf("ERR: Processing: %s, %s", f, err)
			continue
		}
		_, isExtra := extraMap[r.Ext]
		if *mame {
			if r.Ext == ".zip" || r.Ext == ".7z" || isExtra {
				roms <- r
			}
			continue
		}
		_, ok := bins[f]
		if !ok && (rh.KnownExt(r.Ext) || r.Ext == ".svm" || isExtra || r.Ext == ".daphne" || r.Ext == ".7z") {
			roms <- r
		}
	}
	close(roms)
	wg.Wait()
	wg.Add(1)
	close(results)
	wg.Wait()
	if stop {
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
func scrape(sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts) error {
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
	cerr := crawlROMs(gl, sources, xmlOpts, gameOpts)
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
		if s.Path[:2] == prefix {
			s.Path = filepath.Join(hd, s.Path[2:])
		}
		out = append(out, s)
	}
	return out, nil
}

func main() {
	flag.Parse()
	if *version {
		fmt.Println(versionStr)
		return
	}
	if *gdbImg != "" {
		log.Println("DEPRECATED: -gdb_img has been deprecated, use -console_img")
		*consoleImg = *gdbImg
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
	}
	var aImg []ds.ImgType
	var cImg []ds.ImgType
	var ssRegions []ds.RegionType
	var ssLangs []ds.LangType
	for _, t := range strings.Split(*mameImg, ",") {
		aImg = append(aImg, ds.ImgType(t))
	}
	for _, t := range strings.Split(*consoleImg, ",") {
		cImg = append(cImg, ds.ImgType(t))
	}
	for _, r := range strings.Split(*region, ",") {
		ssRegions = append(ssRegions, ds.RegionType(r))
	}
	for _, l := range strings.Split(*lang, ",") {
		ssLangs = append(ssLangs, ds.LangType(l))
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
	if *mame && !*scrapeAll {
		*useGDB = false
		*useOVGDB = false
	}
	var hasher *ds.Hasher
	if *useGDB || *useOVGDB || *useSS {
		var err error
		hasher, err = ds.NewHasher(sha1.New, *workers)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	var hm *ds.HashMap
	var err error
	if *hashFile != "" {
		hm, err = ds.FileHashMap(*hashFile)
	} else {
		hm, err = ds.CachedHashMap("")
	}
	if err != nil {
		fmt.Println(err)
		return
	}
	if *useGDB {
		if !*skipCheck {
			ok := gdb.IsUp()
			if !ok {
				fmt.Println("It appears that thegamesdb.net isn't up. If you are sure it is use -skip_check to bypass this error.")
				return
			}
		}
		consoleSources = append(consoleSources, &ds.GDB{HM: hm, Hasher: hasher})
	}
	if *useSS {
		dev := ss.DevInfo{ID: "sselph", Password: "ROMHasher20160916", Name: "sselph/scraper"}
		ssDS := &ds.SS{
			HM:     hm,
			Hasher: hasher,
			Dev:    dev,
			Width:  int(*maxWidth),
			Region: ssRegions,
			Lang:   ssLangs,
		}
		consoleSources = append(consoleSources, ssDS)
	}
	if *useOVGDB {
		o, err := ds.NewOVGDB(hasher)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer o.Close()
		consoleSources = append(consoleSources, o)
	}
	consoleSources = append(consoleSources, &ds.ScummVM{HM: hm})
	consoleSources = append(consoleSources, &ds.Daphne{HM: hm})
	consoleSources = append(consoleSources, &ds.NeoGeo{HM: hm})
	if *mame || *scrapeAll {
		mds, err := ds.NewMAME("")
		if err != nil {
			fmt.Println(err)
			return
		}
		defer mds.Close()
		arcadeSources = append(arcadeSources, mds)
	}
	arcadeSources = append(arcadeSources, &ds.NeoGeo{HM: hm})
	if !*scrapeAll {
		var sources []ds.DS
		if *mame {
			sources = arcadeSources
			xmlOpts.ImgPriority = aImg
		} else {
			sources = consoleSources
			xmlOpts.ImgPriority = cImg
		}
		err := scrape(sources, xmlOpts, gameOpts)
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
			err := scrape(sources, xmlOpts, gameOpts)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
}
