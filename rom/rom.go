package rom

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sselph/scraper/ds"
)

var lock chan struct{}

func init() {
	SetMaxImg(runtime.NumCPU())
}

// SetMaxImg sets the maximum number of threads that are allowed to have an open image.
func SetMaxImg(x int) {
	lock = make(chan struct{}, x)
	for i := 0; i < x; i++ {
		lock <- struct{}{}
	}
}

// GameOpts represents the options for creating Game information.
type GameOpts struct {
	// AddNotFound instructs the scraper to create a Game even if the game isn't in the sources.
	AddNotFound bool
	// NoPrettyName instructs the scraper to leave the name as the name in the source.
	NoPrettyName bool
	// UseFilename instructs the scraper to use the filename minus extension as the xml name.
	UseFilename bool
	// NoStripUnicode instructs the scraper to not strip out unicode characters.
	NoStripUnicode bool
	// OverviewLen is the max length allowed for a overview. 0 means no limit.
	OverviewLen int
}

// XMLOpts represents the options for creating XML information.
type XMLOpts struct {
	// RomDir is the base directory for scraping rom files.
	RomDir string
	// RomXMLDir is the base directory where roms will be located on the target system.
	RomXMLDir string
	// NestImgDir if true tells the scraper to use the same directory structure of roms for rom images.
	NestImgDir bool
	// ImgDir is the base directory for downloading images.
	ImgDir string
	// ImgXMLDir is the directory where images will be located on the target system.
	ImgXMLDir string
	// ImgPriority is the order or image preference when multiple images are avialable.
	ImgPriority []ds.ImgType
	// ImgSuffix is what will be appened to the end of the rom's name to name the image
	// ie rom.bin with suffix of "-image" results in rom-image.jpg
	ImgSuffix string
	// ThumbOnly tells the scraper to prefer thumbnail size images when available.
	ThumbOnly bool
	// NoDownload tells the scraper to not download images.
	NoDownload bool
	// ImgFormat is the format for the image, currently only "jpg" and "png" are supported.
	ImgFormat string
	// ImgWidth is the max width of images. Anything larger will be resized.
	ImgWidth uint
	// ImgHeight is the max height of images. Anything larger will be resized.
	ImgHeight uint
}

// stripChars strips out unicode and converts "fancy" quotes to normal quotes.
func stripChars(r rune) rune {
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

// scanWords is a split function for a Scanner that returns each
// space-separated word of text, with surrounding spaces deleted. It will
// never return an empty string. The definition of space is set by
// unicode.IsSpace.
func scanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading spaces.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !unicode.IsSpace(r) {
			break
		}
	}
	quote := false
	// Scan until space, marking end of word.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		switch {
		case i == 0 && r == '"':
			quote = true
		case !quote && unicode.IsSpace(r):
			return i + width, data[start:i], nil
		case quote && r == '"':
			return i + width, data[start+width : i], nil
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return start, nil, nil
}

// ROM stores information about the ROM.
type ROM struct {
	Path     string
	Dir      string
	BaseName string
	FileName string
	Ext      string
	Bins     []string
	Cue      bool
	Game     *ds.Game
	NotFound bool
}

// populatePaths populates all the relative path information from the full path.
func (r *ROM) populatePaths() {
	r.Dir, r.FileName = filepath.Split(r.Path)
	r.Ext = filepath.Ext(r.FileName)
	r.BaseName = r.FileName[:len(r.FileName)-len(r.Ext)]
}

// populateBins populates .bin information for .cue or .gdi files.
func (r *ROM) populateBins() error {
	f, err := os.Open(r.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	switch {
	case r.Ext == ".gdi":
		if !s.Scan() {
			return fmt.Errorf("bad gdi")
		}
		for s.Scan() {
			w := bufio.NewScanner(strings.NewReader(s.Text()))
			w.Split(scanWords)
			for i := 0; i < 5; i++ {
				if !w.Scan() {
					return fmt.Errorf("bad gdi")
				}
			}
			bin := w.Text()
			p := filepath.Join(r.Dir, bin)
			if exists(p) {
				r.Bins = append(r.Bins, p)
			}
		}
	case r.Ext == ".cue":
		for s.Scan() {
			w := bufio.NewScanner(strings.NewReader(s.Text()))
			w.Split(scanWords)
			if !w.Scan() {
				continue
			}
			t := w.Text()
			if t != "FILE" {
				continue
			}
			if !w.Scan() {
				continue
			}
			bin := w.Text()
			p := filepath.Join(r.Dir, bin)
			if exists(p) {
				r.Bins = append(r.Bins, p)
			}
		}
	}
	return nil
}

// GetGame attempts to populates the Game from data sources in oder.
func (r *ROM) GetGame(data []ds.DS, opts *GameOpts) error {
	if opts == nil {
		opts = &GameOpts{}
	}
	var err error
	var prettyName string
	var game *ds.Game
	files := []string{r.Path}
	if r.Cue {
		files = append(files, r.Bins...)
	}
Loop:
	for _, file := range files {
		for _, source := range data {
			prettyName = source.GetName(file)
			game, err = source.GetGame(file)
			if err != nil {
				continue
			}
			break Loop
		}
	}
	if game == nil {
		if err == ds.ErrNotFound {
			r.NotFound = true
		}
		if err != ds.ErrNotFound || !opts.AddNotFound {
			return err
		}
		game = &ds.Game{GameTitle: r.BaseName}
	}
	if !opts.NoPrettyName && prettyName != "" {
		game.GameTitle = prettyName
	}
	if opts.UseFilename {
		game.GameTitle = r.BaseName
	}
	if !opts.NoStripUnicode {
		game.Overview = strings.Map(stripChars, game.Overview)
		game.GameTitle = strings.Map(stripChars, game.GameTitle)
	}
	if opts.OverviewLen != 0 && opts.OverviewLen > 0 && len(game.Overview) > opts.OverviewLen+3 {
		game.Overview = game.Overview[:opts.OverviewLen] + "..."
	}
	r.Game = game
	return nil
}

// NewROM creates a new ROM and populates path and bin information.
func NewROM(p string) (*ROM, error) {
	r := &ROM{Path: p}
	r.populatePaths()
	r.Cue = r.Ext == ".cue" || r.Ext == ".gdi"
	if r.Cue {
		err := r.populateBins()
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

// getImgPaths gets the paths to use for images.
func getImgPaths(r *ROM, opts *XMLOpts) string {
	var imgPath string
	if opts.NestImgDir {
		dir := strings.TrimPrefix(r.Dir, opts.RomDir)
		imgPath = filepath.Join(opts.ImgDir, dir)
	} else {
		imgPath = opts.ImgDir
	}
	iName := fmt.Sprintf("%s%s.%s", r.BaseName, opts.ImgSuffix, opts.ImgFormat)
	return filepath.Join(imgPath, iName)
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

var imgDirs = make(map[string]struct{})

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

// getImage gets the image, resizes it and saves it to specified path.
func getImage(dsImg ds.Image, p string, w uint, h uint) error {
	dir := filepath.Dir(p)
	if _, ok := imgDirs[dir]; !ok {
		err := mkDir(dir)
		if err != nil {
			return err
		}
		imgDirs[dir] = struct{}{}
	}
	<-lock
	defer func() {
		lock <- struct{}{}
	}()
	return dsImg.Save(p, w, h)
}

func exists(s string) bool {
	fi, err := os.Stat(s)
	return err == nil && fi.Size() > 0
}

// imgExists checks if an image exists with either format.
func imgExists(p string) (string, bool) {
	if exists(p) {
		return p, true
	}
	e := filepath.Ext(p)
	if e == ".jpg" {
		e = ".png"
	} else {
		e = ".jpg"
	}
	op := p[:len(p)-len(e)] + e
	if exists(op) {
		return op, true
	}
	return p, false
}

// XML creates the XML for the ROM after the Game has been populates.
func (r *ROM) XML(opts *XMLOpts) (*GameXML, error) {
	gxml := &GameXML{
		Path:        fixPath(opts.RomXMLDir + "/" + strings.TrimPrefix(r.Path, opts.RomDir)),
		ID:          r.Game.ID,
		GameTitle:   r.Game.GameTitle,
		Overview:    r.Game.Overview,
		Rating:      r.Game.Rating,
		ReleaseDate: r.Game.ReleaseDate,
		Developer:   r.Game.Developer,
		Publisher:   r.Game.Publisher,
		Genre:       r.Game.Genre,
		Source:      r.Game.Source,
	}
	if r.Game.Players > 0 {
		gxml.Players = strconv.FormatInt(r.Game.Players, 10)
	}
	imgPath := getImgPaths(r, opts)
	imgPath, exists := imgExists(imgPath)
	if exists {
		gxml.Image = fixPath(opts.ImgXMLDir + "/" + strings.TrimPrefix(imgPath, opts.ImgDir))
		return gxml, nil
	}
	if opts.NoDownload {
		return gxml, nil
	}
	for _, it := range opts.ImgPriority {
		var dsImg ds.Image
		var ok bool
		if opts.ThumbOnly {
			dsImg, ok = r.Game.Thumbs[it]
		} else {
			dsImg, ok = r.Game.Images[it]
		}
		if ok {
			err := getImage(dsImg, imgPath, opts.ImgWidth, opts.ImgHeight)
			if err == ds.ErrImgNotFound {
				continue
			}
			if err != nil {
				return nil, err
			}
			gxml.Image = fixPath(opts.ImgXMLDir + "/" + strings.TrimPrefix(imgPath, opts.ImgDir))
			break
		}

	}
	return gxml, nil
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
	Players     string   `xml:"players,omitempty"`
	PlayCount   string   `xml:"playcount,omitempty"`
	LastPlayed  string   `xml:"lastplayed,omitempty"`
	Favorite    string   `xml:"favorite,omitempty"`
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
