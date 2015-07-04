package rom

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nfnt/resize"
	"github.com/sselph/scraper/ds"
)

type GameOpts struct {
	AddNotFound    bool
	NoPrettyName   bool
	UseFilename    bool
	NoStripUnicode bool
}

type XMLOpts struct {
	RomDir      string
	RomXMLDir   string
	NestImgDir  bool
	ImgDir      string
	ImgXMLDir   string
	ImgPriority []ds.ImgType
	ImgSuffix   string
	ThumbOnly   bool
	NoDownload  bool
	ImgFormat   string
	ImgWidth    uint
}

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

// rom stores information about the ROM.
type ROM struct {
	Path     string
	Dir      string
	BaseName string
	FileName string
	Ext      string
	Bins     []string
	Cue      bool
	Game     *ds.Game
}

func (r *ROM) populatePaths() {
	r.Dir, r.FileName = filepath.Split(r.Path)
	r.Ext = filepath.Ext(r.FileName)
	r.BaseName = r.FileName[:len(r.FileName)-len(r.Ext)]
}

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

func (r *ROM) GetGame(data []ds.DS, opts *GameOpts) error {
	if opts == nil {
		opts = &GameOpts{}
	}
	var err error
	var id string
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
			id, err = source.GetID(file)
			if err != nil {
				continue
			}
			game, err = source.GetGame(id)
			if err != nil {
				continue
			}
			break Loop
		}
	}
	if game == nil {
		if err != ds.NotFoundErr || !opts.AddNotFound {
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
	r.Game = game
	return nil
}

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
	return fmt.Errorf("%s is a file not a directory.", d)
}

// getImage gets the image, resizes it and saves it to specified path.
func getImage(url string, p string, w uint) error {
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
	if w > 0 && uint(img.Bounds().Dx()) > w {
		img = resize.Resize(w, 0, img, resize.Bilinear)
	}
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()
	e := filepath.Ext(p)
	switch e {
	case ".jpg":
		return jpeg.Encode(out, img, nil)
	case ".png":
		return png.Encode(out, img)
	default:
		return fmt.Errorf("Invalid image type.")
	}
}

func exists(s string) bool {
	fi, err := os.Stat(s)
	return !os.IsNotExist(err) && fi.Size() > 0
}

func imgExists(p string) (string, bool) {
	if exists(p) {
		return p, true
	} else {
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
	}
	return p, false
}

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
	imgPath := getImgPaths(r, opts)
	imgPath, exists := imgExists(imgPath)
	if exists {
		gxml.Image = fixPath(opts.ImgXMLDir + "/" + strings.TrimPrefix(imgPath, opts.ImgDir))
		return gxml, nil
	}
	if opts.NoDownload {
		return gxml, nil
	}
	var imgURL string
	var ok bool
	for _, it := range opts.ImgPriority {
		if opts.ThumbOnly {
			if imgURL, ok = r.Game.Thumbs[it]; ok {
				break
			}
		} else {
			if imgURL, ok = r.Game.Images[it]; ok {
				break
			}
		}
	}
	err := getImage(imgURL, imgPath, opts.ImgWidth)
	if err != nil {
		return nil, err
	}
	gxml.Image = fixPath(opts.ImgXMLDir + "/" + strings.TrimPrefix(imgPath, opts.ImgDir))
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
