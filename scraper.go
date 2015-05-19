package main

import (
	"encoding/csv"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"github.com/kr/fs"
	"github.com/mitchellh/go-homedir"
	"github.com/nfnt/resize"
	"github.com/sselph/scraper/gdb"
	"github.com/sselph/scraper/mamedb"
	"github.com/sselph/scraper/ovgdb"
	"github.com/sselph/scraper/rom"
	_ "github.com/sselph/scraper/rom/bin"
	_ "github.com/sselph/scraper/rom/gb"
	_ "github.com/sselph/scraper/rom/lnx"
	_ "github.com/sselph/scraper/rom/md"
	_ "github.com/sselph/scraper/rom/n64"
	_ "github.com/sselph/scraper/rom/nes"
	_ "github.com/sselph/scraper/rom/pce"
	_ "github.com/sselph/scraper/rom/sms"
	_ "github.com/sselph/scraper/rom/snes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hashURL  = "https://storage.googleapis.com/stevenselph.appspot.com/hash.csv"
	hashName = "hash.csv"
	hashMeta = "hash.meta"
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
var useCache = flag.Bool("use_cache", false, "Use sselph backup of thegamesdb.")
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

var imgDirs map[string]struct{}

var NotFound = errors.New("hash not found")

// GetFront gets the front boxart for a Game if it exists.
func GetFront(g gdb.Game) *gdb.Image {
	for _, v := range g.BoxArt {
		if v.Side == "front" {
			return &v
		}
	}
	return nil
}

// ToXMLDate converts a gdb date to the gamelist.xml date.
func ToXMLDate(d string) string {
	switch len(d) {
	case 10:
		t, _ := time.Parse("01/02/2006", d)
		return t.Format("20060102T000000")
	case 4:
		return fmt.Sprintf("%s0101T000000", d)
	}
	return ""
}

func StripChars(r rune) rune {
	// Single Quote
	if r == 8217 || r == 8216 {
		return 39
	}
	// Double Quote
	if r == 8220 || r == 8221 {
		return 34
	}
	// ASCII
	if r < 127 {
		return r
	}
	return -1
}

type HashMap struct {
	Data map[string][]string
}

func (hm *HashMap) GetID(s string) (string, bool) {
	d, ok := hm.Data[s]
	if !ok || d[0] == "" {
		return "", false
	} else {
		return d[0], true
	}
}

func (hm *HashMap) GetName(s string) (string, bool) {
	d, ok := hm.Data[s]
	if !ok || d[1] == "" {
		return "", false
	} else {
		return d[1], true
	}
}

type datasources struct {
	HM    *HashMap
	OVGDB *ovgdb.DB
}

// GameXML is the object used to export the <game> elements of the gamelist.xml.
type GameXML struct {
	XMLName     xml.Name `xml:"game"`
	ID          string   `xml:"id,attr"`
	Source      string   `xml:"source,attr"`
	Path        string   `xml:"path"`
	GameTitle   string   `xml:"name"`
	Overview    string   `xml:"desc"`
	Image       string   `xml:"image,omitempty"`
	Thumb       string   `xml:"thumbnail,omitempty"`
	Rating      float64  `xml:"rating,omitempty"`
	ReleaseDate string   `xml:"releasedate"`
	Developer   string   `xml:"developer"`
	Publisher   string   `xml:"publisher"`
	Genre       string   `xml:"genre"`
	Players     int64    `xml:"players,omitempty"`
}

// GameListXML is the structure used to export the gamelist.xml file.
type GameListXML struct {
	XMLName  xml.Name   `xml:"gameList"`
	GameList []*GameXML `xml:"game"`
}

// Append appeads a GameXML to the GameList.
func (gl *GameListXML) Append(g *GameXML) {
	gl.GameList = append(gl.GameList, g)
}

// fixPaths fixes relative file paths to include the leading './'.
func fixPath(s string) string {
	s = filepath.ToSlash(s)
	s = strings.Replace(s, "//", "/", -1)
	if filepath.IsAbs(s) || s[0] == '.' || s[0] == '~' {
		return s
	}
	return fmt.Sprintf("./%s", s)
}

func GetImgPaths(r *ROM) (iPath, tPath string) {
	var imgPath string
	if *nestedImageDir {
		imgPath = path.Join(*imageDir, r.fDir)
	} else {
		imgPath = *imageDir
	}
	iName := fmt.Sprintf("%s%s.%s", r.bName, *imageSuffix, *imgFormat)
	iPath = path.Join(imgPath, iName)
	tName := fmt.Sprintf("%s%s.%s", r.bName, *thumbSuffix, *imgFormat)
	tPath = path.Join(imgPath, tName)
	return iPath, tPath
}

func GetGDBGame(r *ROM, ds *datasources) (*GameXML, error) {
	id, ok := ds.HM.GetID(r.Hash)
	if !ok {
		return nil, NotFound
	}
	req := gdb.GGReq{ID: id, Cache: *useCache}
	resp, err := gdb.GetGame(req)
	if err != nil {
		return nil, err
	}
	if len(resp.Game) == 0 {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}
	game := resp.Game[0]
	imageURL := resp.ImageURL
	imgPriority := strings.Split(*gdbImg, ",")
	var iURL, tURL string
Loop:
	for _, i := range imgPriority {
		switch i {
		case "s":
			if len(game.Screenshot) != 0 {
				iURL = game.Screenshot[0].Original.URL
				tURL = game.Screenshot[0].Thumb
				break Loop
			}
		case "b":
			front := GetFront(game)
			if front != nil {
				iURL = front.URL
				tURL = front.Thumb
				break Loop
			}
		case "f":
			if len(game.FanArt) != 0 {
				iURL = game.FanArt[0].Original.URL
				tURL = game.FanArt[0].Thumb
				break Loop
			}
		case "a":
			if len(game.Banner) != 0 {
				iURL = game.Banner[0].URL
				tURL = game.Banner[0].URL
				break Loop
			}
		case "l":
			if len(game.ClearLogo) != 0 {
				iURL = game.ClearLogo[0].URL
				tURL = game.ClearLogo[0].URL
				break Loop
			}
		}
	}
	iPath, tPath := GetImgPaths(r)

	if iURL != "" && *downloadImages {
		switch {
		case !*thumbOnly && !*noThumb:
			err = getImage(imageURL+iURL, iPath)
			if err != nil {
				return nil, err
			}
			err = getImage(imageURL+tURL, tPath)
			if err != nil {
				return nil, err
			}
		case *thumbOnly && !*noThumb:
			err = getImage(imageURL+tURL, tPath)
			if err != nil {
				return nil, err
			}
			iPath = tPath
		case !*thumbOnly && *noThumb:
			err = getImage(imageURL+iURL, iPath)
			if err != nil {
				return nil, err
			}
			tPath = ""
		case *thumbOnly && *noThumb:
			err = getImage(imageURL+tURL, tPath)
			if err != nil {
				return nil, err
			}
			iPath = tPath
			tPath = ""
		}
	} else {
		iPath = ""
		tPath = ""
	}

	var genre string
	if len(game.Genres) >= 1 {
		genre = game.Genres[0]
	}
	gxml := &GameXML{
		Path:        fixPath(*romPath + "/" + strings.TrimPrefix(r.Path, *romDir)),
		ID:          game.ID,
		GameTitle:   game.GameTitle,
		Overview:    game.Overview,
		Rating:      game.Rating / 10.0,
		ReleaseDate: ToXMLDate(game.ReleaseDate),
		Developer:   game.Developer,
		Publisher:   game.Publisher,
		Genre:       genre,
		Source:      "theGamesDB.net",
	}
	if iPath != "" {
		gxml.Image = fixPath(*imagePath + "/" + strings.TrimPrefix(iPath, *imageDir))
	}
	if tPath != "" {
		gxml.Thumb = fixPath(*imagePath + "/" + strings.TrimPrefix(tPath, *imageDir))
	}
	if *useNoIntroName {
		n, ok := ds.HM.GetName(r.Hash)
		if ok {
			gxml.GameTitle = n
		}
	}
	p, err := strconv.ParseInt(strings.TrimRight(game.Players, "+"), 10, 32)
	if err == nil {
		gxml.Players = p
	}
	return gxml, nil
}

func GetOVGDBGame(r *ROM, ds *datasources) (*GameXML, error) {
	g, err := ds.OVGDB.GetGame(r.Hash)
	if err != nil {
		return nil, err
	}
	iPath, _ := GetImgPaths(r)
	if g.Art != "" && *downloadImages {
		err = getImage(g.Art, iPath)
		if err != nil {
			return nil, err
		}
	} else {
		iPath = ""
	}

	gxml := &GameXML{
		ID:          g.ReleaseID,
		Path:        fixPath(*romPath + "/" + strings.TrimPrefix(r.Path, *romDir)),
		GameTitle:   g.Name,
		Overview:    g.Desc,
		ReleaseDate: g.Date,
		Developer:   g.Developer,
		Publisher:   g.Publisher,
		Genre:       g.Genre,
		Source:      g.Source,
	}
	if iPath != "" {
		gxml.Image = fixPath(*imagePath + "/" + strings.TrimPrefix(iPath, *imageDir))
	}
	return gxml, nil
}

func GetMAMEGame(r *ROM) (*GameXML, error) {
	g, err := mamedb.GetGame(r.bName, strings.Split(*mameImg, ","))
	if err != nil {
		return nil, err
	}
	iPath, _ := GetImgPaths(r)
	if g.Art != "" && *downloadImages {
		err = getImage(g.Art, iPath)
		if err != nil {
			return nil, err
		}
	} else {
		iPath = ""
	}
	gxml := &GameXML{
		Path:        fixPath(*romPath + "/" + strings.TrimPrefix(r.Path, *romDir)),
		ID:          g.ID,
		GameTitle:   g.Name,
		ReleaseDate: g.Date,
		Developer:   g.Developer,
		Genre:       g.Genre,
		Source:      g.Source,
		Players:     g.Players,
		Rating:      g.Rating / 10.0,
	}
	if iPath != "" {
		gxml.Image = fixPath(*imagePath + "/" + strings.TrimPrefix(iPath, *imageDir))
	}
	return gxml, nil
}

// rom stores information about the ROM.
type ROM struct {
	Path  string
	Hash  string
	bName string
	fName string
	fDir  string
	XML   *GameXML
}

// ProcessROM does all the processing of the ROM. It hashes,
// downloads the metadata, and downloads the images.
// The results are stored in the ROMs XML property.
func (r *ROM) ProcessROM(ds *datasources) error {
	log.Printf("INFO: Starting: %s", r.Path)
	f := filepath.Base(r.Path)
	r.fDir = strings.TrimPrefix(filepath.Dir(r.Path), *romDir)
	r.fName = f
	e := path.Ext(f)
	r.bName = f[:len(f)-len(e)]
	var xml *GameXML
	var err error
	if *mame {
		log.Printf("INFO: Attempting lookup in MAMEDB: %s", r.Path)
		xml, err = GetMAMEGame(r)
	} else {
		h, err := rom.SHA1(r.Path)
		if err != nil {
			return err
		}
		r.Hash = h
	}
	if xml == nil && *useGDB {
		log.Printf("INFO: Attempting lookup in GDB: %s", r.Path)
		xml, err = GetGDBGame(r, ds)
	}
	if xml == nil && *useOVGDB {
		log.Printf("INFO: Attempting lookup in OVGDB: %s", r.Path)
		xml, err = GetOVGDBGame(r, ds)
	}
	if err != nil {
		if err == ovgdb.NotFound {
			err = NotFound
		}
		if err == mamedb.NotFound {
			err = NotFound
		}
		if *addNotFound && err == NotFound {
			log.Printf("INFO: %s: %s", r.Path, err)
			xml = &GameXML{Path: fixPath(*romPath + "/" + strings.TrimPrefix(r.Path, *romDir)), GameTitle: r.bName}
			if *useNoIntroName && *useGDB {
				n, ok := ds.HM.GetName(r.Hash)
				if ok {
					xml.GameTitle = n
				}
			}
		} else {
			return err
		}
	}
	if *useFilename {
		xml.GameTitle = r.bName
	}
	if *stripUnicode {
		xml.Overview = strings.Map(StripChars, xml.Overview)
		xml.GameTitle = strings.Map(StripChars, xml.GameTitle)
	}
	iPath, tPath := GetImgPaths(r)
	iExists := exists(iPath)
	tExists := exists(tPath)
	if xml.Image == "" && iExists {
		xml.Image = fixPath(*imagePath + "/" + strings.TrimPrefix(iPath, *imageDir))
	}
	if xml.Thumb == "" && tExists {
		xml.Thumb = fixPath(*imagePath + "/" + strings.TrimPrefix(tPath, *imageDir))
	}
	r.XML = xml
	return nil
}

// getImage gets the image, resizes it and saves it to specified path.
func getImage(url string, p string) error {
	if exists(p) {
		log.Printf("INFO: Skipping %s", p)
		return nil
	}
	dir := filepath.Dir(p)
	if _, ok := imgDirs[dir]; !ok {
		err := mkDir(dir)
		if err != nil {
			return err
		}
		imgDirs[dir] = struct{}{}
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if uint(img.Bounds().Dx()) > *maxWidth {
		img = resize.Resize(*maxWidth, 0, img, resize.Bilinear)
	}
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()
	switch *imgFormat {
	case "jpg":
		return jpeg.Encode(out, img, nil)
	case "png":
		return png.Encode(out, img)
	default:
		return fmt.Errorf("Invalid image type.")
	}
}

// exists checks if a file exists and contains data.
func exists(s string) bool {
	fi, err := os.Stat(s)
	return !os.IsNotExist(err) && fi.Size() > 0
}

func validTemp(s string) bool {
	fi, err := os.Stat(s)
	if err != nil || fi.Size() == 0 {
		return false
	}
	n := time.Now()
	t := fi.ModTime().Add(30 * time.Minute)
	if t.Before(n) {
		return false
	}
	return true
}

// worker is a function to process roms from a channel.
func worker(ds *datasources, results chan *GameXML, roms chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	var stop bool
	go func() {
		<-sig
		stop = true
	}()
	for p := range roms {
		if stop {
			continue
		}
		r := ROM{Path: p}
		for try := 0; try <= *retries; try++ {
			if stop {
				break
			}
			err := r.ProcessROM(ds)
			if err != nil {
				log.Printf("ERR: error processing %s: %s", r.Path, err)
				if err == NotFound {
					break
				} else {
					continue
				}
			}
			results <- r.XML
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
func CrawlROMs(gl *GameListXML, ds *datasources) error {
	var t = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	var ct http.RoundTripper = NewCancelTransport(t)
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
	results := make(chan *GameXML, *workers)
	roms := make(chan string, 2**workers)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(ds, results, roms, &wg)
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
		if *mame {
			e := path.Ext(f)
			if e == ".zip" || e == ".7z" {
				roms <- f
			}
			continue
		}
		if rom.KnownExt(path.Ext(f)) {
			roms <- f
		}
	}
	close(roms)
	wg.Wait()
	wg.Add(1)
	close(results)
	wg.Wait()
	return nil
}

func updateHash(version, p string) error {
	log.Print("INFO: Checking for new hash.csv.")
	req, err := http.NewRequest("GET", hashURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("if-none-match", version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		log.Printf("INFO: hash.csv %s up to date.", version)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got %v response", resp.Status)
	}
	newVersion := resp.Header.Get("etag")
	log.Printf("INFO: Upgrading hash.csv: %s -> %s.", version, newVersion)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(p, hashName), b, 0664)
	if err != nil {
		return err
	}
	ioutil.WriteFile(path.Join(p, hashMeta), []byte(newVersion), 0664)
	return nil
}

// GetMap gets the mapping of hashes to IDs.
func GetHashMap() (*HashMap, error) {
	ret := &HashMap{Data: make(map[string][]string)}
	var f io.ReadCloser
	var err error
	if *hashFile != "" {
		f, err = os.Open(*hashFile)
		if err != nil {
			return ret, err
		}
	} else {
		p, err := ovgdb.GetDBPath()
		if err != nil {
			return ret, err
		}
		err = mkDir(p)
		if err != nil {
			return ret, err
		}
		fp := path.Join(p, hashName)
		mp := path.Join(p, hashMeta)
		var version string
		if exists(fp) && exists(mp) {
			b, err := ioutil.ReadFile(mp)
			if err != nil {
				return nil, err
			}
			version = strings.Trim(string(b[:]), "\n\r")
		}
		err = updateHash(version, p)
		if err != nil {
			return ret, err
		}
		f, err = os.Open(fp)
		if err != nil {
			return ret, err
		}
	}
	defer f.Close()
	c := csv.NewReader(f)
	r, err := c.ReadAll()
	if err != nil {
		return ret, err
	}
	for _, v := range r {
		ret.Data[strings.ToLower(v[0])] = []string{v[1], v[3]}
	}
	return ret, nil
}

// mkDir checks if directory exists and if it doesn't create it.
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
	return fmt.Errorf("%s is a file not a directory.", d)
}

func Scrape(ds *datasources) error {
	gl := &GameListXML{}
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
	CrawlROMs(gl, ds)
	output, err := xml.MarshalIndent(gl, "  ", "    ")
	if err != nil {
		return err
	}
	if len(gl.GameList) == 0 {
		return nil
	}
	err = ioutil.WriteFile(*outputFile, output, 0664)
	if err != nil {
		return err
	}
	return nil
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
	runtime.GOMAXPROCS(runtime.NumCPU())
	if *startPprof {
		go http.ListenAndServe(":8080", nil)
	}
	imgDirs = make(map[string]struct{})
	ds := &datasources{}
	if *mame {
		*useGDB = false
		*useOVGDB = false
	}
	if *useGDB {
		if !(*skipCheck || *useCache) {
			ok := gdb.IsUp()
			if !ok {
				fmt.Println("It appears that thegamesdb.net isn't up, try -use_cache to use my backup server. If you are sure it is use -skip_check to bypass this error.")
				return
			}
		}
		hm, err := GetHashMap()
		if err != nil {
			fmt.Println(err)
			return
		}
		ds.HM = hm
	}
	if *useOVGDB {
		o, err := ovgdb.GetDB()
		if err != nil {
			fmt.Println(err)
			return
		}
		defer o.Close()
		ds.OVGDB = o
	}
	if !*scrapeAll {
		err := Scrape(ds)
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
			err := Scrape(ds)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
}
