package main

import (
	"encoding/csv"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"github.com/kr/fs"
	"github.com/nfnt/resize"
	"github.com/sselph/scraper/gdb"
	"github.com/sselph/scraper/ovgdb"
	"github.com/sselph/scraper/rom"
	_ "github.com/sselph/scraper/rom/bin"
	_ "github.com/sselph/scraper/rom/gb"
	_ "github.com/sselph/scraper/rom/md"
	_ "github.com/sselph/scraper/rom/nes"
	_ "github.com/sselph/scraper/rom/pce"
	_ "github.com/sselph/scraper/rom/sms"
	_ "github.com/sselph/scraper/rom/snes"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"path/filepath"
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
var romPath = flag.String("rom_path", ".", "The path to use for roms in gamelist.xml.")
var maxWidth = flag.Uint("max_width", 400, "The max width of images. Larger images will be resized.")
var workers = flag.Int("workers", 1, "The number of worker threads used to process roms.")
var retries = flag.Int("retries", 2, "The number of times to retry a rom on an error.")
var thumbOnly = flag.Bool("thumb_only", false, "Download the thumbnail for both the image and thumb (faster).")
var skipCheck = flag.Bool("skip_check", false, "Skip the check if thegamesdb.net is up.")
var useCache = flag.Bool("use_cache", false, "Use sselph backup of thegamesdb.")
var nestedImageDir = flag.Bool("nested_img_dir", false, "Use a nested img directory structure that matches rom structure.")
var useGDB = flag.Bool("use_gdb", true, "Use the hash.csv and theGamesDB metadata.")
var useOVGDB = flag.Bool("use_ovgdb", true, "Use the OpenVGDB if the hash isn't in hash.csv.")
var startPprof = flag.Bool("start_pprof", false, "If true, start the pprof service used to profile the application.")

var imgDirs map[string]struct{}

var HashNotFound = errors.New("hash not found")

// GetFront gets the front boxart for a Game if it exists.
func GetFront(g gdb.Game) (gdb.Image, error) {
	for _, v := range g.BoxArt {
		if v.Side == "front" {
			return v, nil
		}
	}
	return gdb.Image{}, fmt.Errorf("no image for %s", g.GameTitle)
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

type datasources struct {
	HM    map[string]string
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
	XMLName  xml.Name `xml:"gameList"`
	GameList []*GameXML
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

func GetGDBGame(r *ROM, ds *datasources) (*GameXML, error) {
	id, ok := ds.HM[r.Hash]
	if !ok {
		return nil, HashNotFound
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
	front, err := GetFront(game)
	if err != nil {
		return nil, err
	}
	var imgPath string
	if *nestedImageDir {
		imgPath = path.Join(*imageDir, r.fDir)
	} else {
		imgPath = *imageDir
	}
	var iPath string
	if !*thumbOnly {
		iName := fmt.Sprintf("%s-image.jpg", r.bName)
		iPath = path.Join(imgPath, iName)
		err = getImage(imageURL+front.URL, iPath)
		if err != nil {
			return nil, err
		}
	}
	tName := fmt.Sprintf("%s-thumb.jpg", r.bName)
	tPath := path.Join(imgPath, tName)
	if *thumbOnly {
		iPath = tPath
	}
	err = getImage(imageURL+front.Thumb, tPath)
	if err != nil {
		return nil, err
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
	var imgPath string
	if *nestedImageDir {
		imgPath = path.Join(*imageDir, r.fDir)
	} else {
		imgPath = *imageDir
	}
	var iPath string
	if g.Art != "" {
		iName := fmt.Sprintf("%s-image.jpg", r.bName)
		iPath = path.Join(imgPath, iName)
		err = getImage(g.Art, iPath)
		if err != nil {
			return nil, err
		}
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
		gxml.Thumb = fixPath(*imagePath + "/" + strings.TrimPrefix(iPath, *imageDir))
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
	h, err := rom.SHA1(r.Path)
	if err != nil {
		return err
	}
	r.Hash = h
	var xml *GameXML
	if xml == nil && *useGDB {
		log.Printf("INFO: Attempting lookup in GDB: %s", r.Path)
		xml, err = GetGDBGame(r, ds)
	}
	if xml == nil && *useOVGDB {
		log.Printf("INFO: Attempting lookup in OVGDB: %s", r.Path)
		xml, err = GetOVGDBGame(r, ds)
	}
	if err != nil {
		return err
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
	return jpeg.Encode(out, img, nil)
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
	for p := range roms {
		r := ROM{Path: p}
		for try := 0; try <= *retries; try++ {
			err := r.ProcessROM(ds)
			if err != nil {
				log.Printf("ERR: error processing %s: %s", r.Path, err)
				continue
			}
			results <- r.XML
			break
		}
	}
}

// CrawlROMs crawls the rom directory and processes the files.
func CrawlROMs(gl *GameListXML, ds *datasources) error {
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
	walker := fs.Walk(*romDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		f := walker.Path()
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
func GetHashMap() (map[string]string, error) {
	ret := make(map[string]string)
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
		ret[v[0]] = v[1]
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

func main() {
	flag.Parse()
	if *startPprof {
		go http.ListenAndServe(":8080", nil)
	}
	imgDirs = make(map[string]struct{})
	if !*skipCheck {
		ok := gdb.IsUp()
		if !ok {
			fmt.Println("It appears that thegamesdb.net isn't up, try -use_cache to use my backup server. If you are sure it is use -skip_check to bypass this error.")
			return
		}
	}
	ds := &datasources{}
	if *useGDB {
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
	gl := &GameListXML{}
	CrawlROMs(gl, ds)
	output, err := xml.MarshalIndent(gl, "  ", "    ")
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	err = ioutil.WriteFile(*outputFile, output, 0664)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
}
