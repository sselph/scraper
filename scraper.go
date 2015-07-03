package main

import (
	"crypto/sha1"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"github.com/kr/fs"
	"github.com/mitchellh/go-homedir"
	"github.com/sselph/scraper/ds"
	"github.com/sselph/scraper/gdb"
	"github.com/sselph/scraper/rom"
	rh "github.com/sselph/scraper/rom/hash"
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
)

var hashFile = flag.String("hash_file", "", "The file containing hash information.")
var romDir = flag.String("rom_dir", ".", "The directory containing the roms file to process.")
var outputFile = flag.String("output_file", "gamelist.xml", "The XML file to output to.")
var imageDir = flag.String("image_dir", "images", "The directory to place downloaded images to locally.")
var imagePath = flag.String("image_path", "images", "The path to use for images in gamelist.xml.")
var imageSuffix = flag.String("image_suffix", "-image", "The suffix added after rom name when creating image files.")
var thumbSuffix = flag.String("thumb_suffix", "-thumb", "The suffix added after rom name when creating thumb files.")
var romPath = flag.String("rom_path", ".", "The path to use for roms in gamelist.xml.")
var maxWidth = flag.Uint("max_width", 400, "The max width of images. Larger images will be resized.")
var workers = flag.Int("workers", 1, "The number of worker threads used to process roms.")
var retries = flag.Int("retries", 2, "The number of times to retry a rom on an error.")
var thumbOnly = flag.Bool("thumb_only", false, "Download the thumbnail for both the image and thumb (faster).")
var noThumb = flag.Bool("no_thumb", false, "Don't add thumbnails to the gamelist.")
var skipCheck = flag.Bool("skip_check", false, "Skip the check if thegamesdb.net is up.")
var nestedImageDir = flag.Bool("nested_img_dir", false, "Use a nested img directory structure that matches rom structure.")
var useGDB = flag.Bool("use_gdb", true, "Use the hash.csv and theGamesDB metadata.")
var useOVGDB = flag.Bool("use_ovgdb", false, "Use the OpenVGDB if the hash isn't in hash.csv.")
var startPprof = flag.Bool("start_pprof", false, "If true, start the pprof service used to profile the application.")
var useFilename = flag.Bool("use_filename", false, "If true, use the filename minus the extension as the game title in xml.")
var addNotFound = flag.Bool("add_not_found", false, "If true, add roms that are not found as an empty gamelist entry.")
var useNoIntroName = flag.Bool("use_nointro_name", true, "Use the name in the No-Intro DB instead of the one in the GDB.")
var mame = flag.Bool("mame", false, "If true we want to run in MAME mode.")
var mameImg = flag.String("mame_img", "s,t,m,c", "Comma seperated order to prefer images, s=snap, t=title, m=marquee, c=cabniet.")
var stripUnicode = flag.Bool("strip_unicode", true, "If true, remove all non-ascii characters.")
var downloadImages = flag.Bool("download_images", true, "If false, don't download any images, instead see if the expected file is stored locally already.")
var scrapeAll = flag.Bool("scrape_all", false, "If true, scrape all systems listed in es_systems.cfg. All dir/path flags will be ignored.")
var gdbImg = flag.String("gdb_img", "b", "Comma seperated order to prefer images, s=snapshot, b=boxart, f=fanart, a=banner, l=logo.")
var imgFormat = flag.String("img_format", "jpg", "jpg or png, the format to write the images.")
var appendOut = flag.Bool("append", false, "If the gamelist file already exist skip files that are already listed and only append new files.")
var version = flag.Bool("version", false, "Print the release version and exit.")

var UserCanceled = errors.New("user canceled")

var versionStr string

// exists checks if a file exists and contains data.
func exists(s string) bool {
	fi, err := os.Stat(s)
	return !os.IsNotExist(err) && fi.Size() > 0
}

// worker is a function to process roms from a channel.
func worker(sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts, results chan *rom.GameXML, roms chan *rom.ROM, wg *sync.WaitGroup) {
	defer wg.Done()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	var stop bool
	go func() {
		<-sig
		stop = true
	}()
	for r := range roms {
		if stop {
			continue
		}
		for try := 0; try <= *retries; try++ {
			if stop {
				break
			}
			log.Printf("INFO: Starting: %s", r.Path)
			err := r.GetGame(sources, gameOpts)
			if err != nil {
				log.Printf("ERR: error processing %s: %s", r.Path, err)
				if err == ds.NotFoundErr {
					break
				} else {
					continue
				}
			}
			xml, err := r.XML(xmlOpts)
			if err != nil {
				log.Printf("ERR: error processing %s: %s", r.Path, err)
				continue
			}
			results <- xml
			break
		}
	}
}

type CancelTransport struct {
	mu      sync.Mutex
	Pending map[*http.Request]struct{}
	T       *http.Transport
	stop    bool
}

func (t *CancelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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

func (t *CancelTransport) Stop() {
	t.mu.Lock()
	t.stop = true
	for req := range t.Pending {
		t.T.CancelRequest(req)
	}
	t.Pending = make(map[*http.Request]struct{})
	t.mu.Unlock()
}

func NewCancelTransport(t *http.Transport) *CancelTransport {
	ct := &CancelTransport{T: t}
	ct.Pending = make(map[*http.Request]struct{})
	return ct
}

// CrawlROMs crawls the rom directory and processes the files.
func CrawlROMs(gl *rom.GameListXML, sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts) error {
	var ct http.RoundTripper = NewCancelTransport(http.DefaultTransport.(*http.Transport))
	http.DefaultClient.Transport = ct

	existing := make(map[string]struct{})

	for _, x := range gl.GameList {
		p, err := filepath.Rel(*romPath, x.Path)
		if err != nil {
			log.Printf("Can't find original path: %s", x.Path)
		}
		f := filepath.Join(*romDir, p)
		existing[f] = struct{}{}
	}

	var wg sync.WaitGroup
	results := make(chan *rom.GameXML, *workers)
	roms := make(chan *rom.ROM, 2**workers)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(sources, xmlOpts, gameOpts, results, roms, &wg)
	}
	go func() {
		defer wg.Done()
		for r := range results {
			gl.Append(r)
		}
	}()
	var stop bool
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		for {
			<-sig
			if !stop {
				stop = true
				log.Println("Stopping, ctrl-c again to stop now.")
				ct.(*CancelTransport).Stop()
				for _ = range roms {
				}
				continue
			}
			panic("AHHHH!")
		}
	}()
	bins := make(map[string]struct{})
	if !*mame {
		walker := fs.Walk(*romDir)
		for walker.Step() {
			if stop {
				break
			}
			if err := walker.Err(); err != nil {
				return err
			}
			f := walker.Path()
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
			if _, ok := existing[f]; ok {
				log.Printf("INFO: Skipping %s, already in gamelist.", f)
				continue
			}
			roms <- r
		}
	}
	walker := fs.Walk(*romDir)
	for walker.Step() {
		if stop {
			break
		}
		if err := walker.Err(); err != nil {
			return err
		}
		f := walker.Path()
		if _, ok := existing[f]; ok {
			log.Printf("INFO: Skipping %s, already in gamelist.", f)
			continue
		}
		r, err := rom.NewROM(f)
		if err != nil {
			log.Printf("ERR: Processing: %s, %s", f, err)
			continue
		}
		if *mame {
			if r.Ext == ".zip" || r.Ext == ".7z" {
				roms <- r
			}
			continue
		}
		_, ok := bins[f]
		if !ok && rh.KnownExt(r.Ext) {
			roms <- r
		}
	}
	close(roms)
	wg.Wait()
	wg.Add(1)
	close(results)
	wg.Wait()
	if stop {
		return UserCanceled
	} else {
		return nil
	}
}

func Scrape(sources []ds.DS, xmlOpts *rom.XMLOpts, gameOpts *rom.GameOpts) error {
	gl := &rom.GameListXML{}
	if *appendOut {
		f, err := os.Open(*outputFile)
		if err != nil {
			log.Printf("ERR: Can't open %s, creating new file.", *outputFile)
		} else {
			decoder := xml.NewDecoder(f)
			if err := decoder.Decode(gl); err != nil {
				log.Printf("ERR: Can't open %s, creating new file.", *outputFile)
			}
			f.Close()
		}
	}
	cerr := CrawlROMs(gl, sources, xmlOpts, gameOpts)
	if cerr != nil && cerr != UserCanceled {
		return cerr
	}
	output, err := xml.MarshalIndent(gl, "  ", "    ")
	if err != nil {
		return err
	}
	if len(gl.GameList) == 0 {
		return cerr
	}
	err = ioutil.WriteFile(*outputFile, append([]byte(xml.Header), output...), 0664)
	if err != nil {
		return err
	}
	return cerr
}

type System struct {
	Name      string `xml:"name"`
	Path      string `xml:"path"`
	Extension string `xml:"extension"`
}

type ESSystems struct {
	XMLName xml.Name `xml:"systemList"`
	Systems []System `xml:"system"`
}

func GetRomFolders() ([]string, error) {
	hd, err := homedir.Dir()
	if err != nil {
		return nil, err
	}
	p := filepath.Join(hd, ".emulationstation", "es_systems.cfg")
	ap := "/etc/emulationstation/es_systems.cfg"
	if !exists(p) && !exists(ap) {
		return nil, fmt.Errorf("%s and %s not found.", p, ap)
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
	var out []string
	prefix := string([]rune{'~', filepath.Separator})
	for _, s := range v.Systems {
		if s.Path[:2] == prefix {
			s.Path = filepath.Join(hd, s.Path[2:])
		}
		out = append(out, s.Path)
	}
	return out, nil
}

func main() {
	flag.Parse()
	if *version {
		fmt.Println(versionStr)
		return
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	if *startPprof {
		go http.ListenAndServe(":8080", nil)
	}
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
	var ifmt string
	if *mame {
		ifmt = *mameImg
	} else {
		ifmt = *gdbImg
	}
	for _, t := range strings.Split(ifmt, ",") {
		xmlOpts.ImgPriority = append(xmlOpts.ImgPriority, ds.ImgType(t))
	}
	gameOpts := &rom.GameOpts{
		AddNotFound:    *addNotFound,
		NoPrettyName:   !*useNoIntroName,
		UseFilename:    *useFilename,
		NoStripUnicode: !*stripUnicode,
	}
	sources := []ds.DS{}
	if *mame {
		*useGDB = false
		*useOVGDB = false
		sources = append(sources, &ds.MAME{})
	}
	if *useGDB {
		if !*skipCheck {
			ok := gdb.IsUp()
			if !ok {
				fmt.Println("It appears that thegamesdb.net isn't up. If you are sure it is use -skip_check to bypass this error.")
				return
			}
		}
		hm, err := ds.CachedHashMap("")
		if err != nil {
			fmt.Println(err)
			return
		}
		h, err := ds.NewHasher(sha1.New)
		if err != nil {
			fmt.Println(err)
			return
		}
		sources = append(sources, &ds.GDB{HM: hm, Hasher: h})
	}
	//	if *useOVGDB {
	//		o, err := ovgdb.GetDB()
	//		if err != nil {
	//			fmt.Println(err)
	//			return
	//		}
	//		defer o.Close()
	//		ds.OVGDB = o
	//	}
	if !*scrapeAll {
		err := Scrape(sources, xmlOpts, gameOpts)
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		romFolders, err := GetRomFolders()
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, rf := range romFolders {
			log.Printf("Starting System %s", rf)
			*romDir = rf
			*romPath = rf
			p := filepath.Join(rf, "images")
			*imageDir = p
			*imagePath = p
			out := filepath.Join(rf, "gamelist.xml")
			*outputFile = out
			err := Scrape(sources, xmlOpts, gameOpts)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
}
