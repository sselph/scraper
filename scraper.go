package main

import (
	"encoding/csv"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/kr/fs"
	"github.com/nfnt/resize"
	"github.com/sselph/scraper/gdb"
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
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hashURL      = "https://stevenselph.appspot.com/csv/hash.csv"
	tempHashFile = "hash.csv"
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

var imgDirs map[string]struct{}

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

// GameXML is the object used to export the <game> elements of the gamelist.xml.
type GameXML struct {
	XMLName     xml.Name `xml:"game"`
	ID          string   `xml:"id,attr"`
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
	GameList []GameXML
}

// Append appeads a GameXML to the GameList.
func (gl *GameListXML) Append(g GameXML) {
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

// rom stores information about the ROM.
type ROM struct {
	Path     string
	hash     string
	id       string
	game     gdb.Game
	imageURL string
	iPath    string
	tPath    string
	bName    string
	fName    string
	fDir     string
	iTag     string
	tTag     string
	XML      GameXML
}

func (r *ROM) getID(hm map[string]string) error {
	var ok bool
	h, err := rom.SHA1(r.Path)
	if err != nil {
		return err
	}
	r.id, ok = hm[h]
	if !ok {
		return fmt.Errorf("hash for file %s not found", r.Path)
	}
	return nil
}

// getGame gets the game information from the DB.
func (r *ROM) getGame() error {
	req := gdb.GGReq{ID: r.id, Cache: *useCache}
	resp, err := gdb.GetGame(req)
	if err != nil {
		return err
	}
	if len(resp.Game) == 0 {
		return fmt.Errorf("game with id (%s) not found", r.id)
	}
	r.game = resp.Game[0]
	r.imageURL = resp.ImageURL
	return nil
}

// ProcessROM does all the processing of the ROM. It hashes,
// downloads the metadata, and downloads the images.
// The results are stored in the ROMs XML property.
func (r *ROM) ProcessROM(hm map[string]string) error {
	log.Printf("INFO: Starting: %s", r.Path)
	f := filepath.Base(r.Path)
	r.fDir = strings.TrimPrefix(filepath.Dir(r.Path), *romDir)
	r.fName = f
	e := path.Ext(f)
	r.bName = f[:len(f)-len(e)]
	err := r.getID(hm)
	if err != nil {
		return err
	}
	err = r.getGame()
	if err != nil {
		return err
	}
	err = r.downloadImages()
	if err != nil {
		return err
	}
	var genre string
	if len(r.game.Genres) >= 1 {
		genre = r.game.Genres[0]
	}

	r.XML = GameXML{
		Path:        fixPath(*romPath + "/" + strings.TrimPrefix(r.Path, *romDir)),
		ID:          r.game.ID,
		GameTitle:   r.game.GameTitle,
		Overview:    r.game.Overview,
		Rating:      r.game.Rating / 10.0,
		ReleaseDate: ToXMLDate(r.game.ReleaseDate),
		Developer:   r.game.Developer,
		Publisher:   r.game.Publisher,
		Genre:       genre,
	}
	if r.iPath != "" {
		r.XML.Image = fixPath(*imagePath + "/" + strings.TrimPrefix(r.iPath, *imageDir))
	}
	if r.tPath != "" {
		r.XML.Thumb = fixPath(*imagePath + "/" + strings.TrimPrefix(r.tPath, *imageDir))
	}
	p, err := strconv.ParseInt(strings.TrimRight(r.game.Players, "+"), 10, 32)
	if err == nil {
		r.XML.Players = p
	}
	return nil
}

// downloadImages downloads the ROMs images.
func (r *ROM) downloadImages() error {
	f, err := GetFront(r.game)
	if err != nil {
		return err
	}
	var imgPath string
	if *nestedImageDir {
		imgPath = path.Join(*imageDir, r.fDir)
	} else {
		imgPath = *imageDir
	}
	if _, ok := imgDirs[imgPath]; !ok {
		err := mkDir(imgPath)
		if err != nil {
			return err
		}
		imgDirs[imgPath] = struct{}{}
	}
	if !*thumbOnly {
		iName := fmt.Sprintf("%s-image.jpg", r.bName)
		r.iPath = path.Join(imgPath, iName)
		if !exists(r.iPath) {
			err = getImage(r.imageURL+f.URL, r.iPath)
			if err != nil {
				return err
			}
		} else {
			log.Printf("INFO: Skipping %s", r.iPath)
		}
	}
	tName := fmt.Sprintf("%s-thumb.jpg", r.bName)
	r.tPath = path.Join(imgPath, tName)
	if *thumbOnly {
		r.iPath = r.tPath
	}
	if !exists(r.tPath) {
		err = getImage(r.imageURL+f.Thumb, r.tPath)
		if err != nil {
			return err
		}
	} else {
		log.Printf("INFO: Skipping %s", r.tPath)
	}
	return nil
}

// getImage gets the image, resizes it and saves it to specified path.
func getImage(url string, p string) error {
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
func worker(hm map[string]string, results chan GameXML, roms chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for p := range roms {
		r := ROM{Path: p}
		for try := 0; try <= *retries; try++ {
			err := r.ProcessROM(hm)
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
func CrawlROMs(gl *GameListXML, hm map[string]string) error {
	var wg sync.WaitGroup
	results := make(chan GameXML, *workers)
	roms := make(chan string, 2**workers)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(hm, results, roms, &wg)
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

// GetMap gets the mapping of hashes to IDs.
func GetHashMap() (map[string]string, error) {
	ret := make(map[string]string)
	var f io.ReadCloser
	var err error
	tempFile := path.Join(os.TempDir(), tempHashFile)
	switch {
	case *hashFile != "":
		f, err = os.Open(*hashFile)
		if err != nil {
			return ret, err
		}
	case validTemp(tempFile):
		f, err = os.Open(tempFile)
		if err != nil {
			return ret, err
		}
	default:
		resp, err := http.Get(hashURL)
		if err != nil {
			return ret, err
		}
		defer resp.Body.Close()
		r, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return ret, err
		}
		err = ioutil.WriteFile(tempFile, r, 0644)
		if err != nil {
			return ret, err
		}
		f, err = os.Open(tempFile)
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
		return os.MkdirAll(d, 0777)
	case err != nil:
		return err
	case fi.IsDir():
		return nil
	}
	return fmt.Errorf("%s is a file not a directory.", d)
}

func main() {
	imgDirs = make(map[string]struct{})
	flag.Parse()
	if !*skipCheck {
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
	gl := &GameListXML{}
	CrawlROMs(gl, hm)
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
