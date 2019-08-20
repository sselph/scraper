package rom

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sselph/scraper/ds"
)

var lock chan struct{}
var imgExts = []string{".jpg", ".png"}
var vidExts = []string{".mp4", ".mkv", ".flv", ".ogv", ".avi", ".mov", ".mpg", "m4v"}

func init() {
	SetMaxImg(runtime.NumCPU())
	if runtime.GOOS == "windows" {
		os.Setenv("PATH", fmt.Sprintf("C:\\Program Files\\FFmpeg;%s", os.Getenv("PATH")))
	}
}

// SetMaxImg sets the maximum number of threads that are allowed to have an open image.
func SetMaxImg(x int) {
	lock = make(chan struct{}, x)
	for i := 0; i < x; i++ {
		lock <- struct{}{}
	}
}

// stripChars strips out unicode and converts "fancy" quotes to normal quotes.
func stripChars(r rune) rune {
	switch {
	case r == 8217 || r == 8216:
		return 39 // Single Quote
	case r == 8220 || r == 8221:
		return 34 // Double Quote
	case r < 127:
		return r
	}
	return -1
}

// stripCharsForFilename strips out or replace forbidden chars in filenames
func stripCharsForFilename(r rune) rune {
	switch {
	case r == 8217 || r == 8216 || r == 8220 || r == 8221 || r == 63 || r == 42:
		return 32
	case r == 58 || r == 47 || r == 92 || r == 124:
		return 45 // hyphen
	case r < 127:
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
	Path      string
	Dir       string
	BaseName  string
	FileName  string
	CleanName string
	Ext       string
	Bins      []string
	Cue       bool

	Game       *ds.Game
	NotFound   bool
	NumRetries int
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
func (r *ROM) GetGame(ctx context.Context, data []ds.DS, opts *GameOpts) error {
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
			game, err = source.GetGame(ctx, file)
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
	r.CleanName = strings.Map(stripCharsForFilename, game.GameTitle)
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
	if opts.OverviewLen > 0 && len(game.Overview) > opts.OverviewLen+3 {
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
		if err := r.populateBins(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// getImgPath gets the path to use for images.
func getImgPath(r *ROM, opts *XMLOpts) string {
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

// getVidPath gets the path to use for video.
func getVidPath(r *ROM, opts *XMLOpts) string {
	var vidPath string
	if opts.NestImgDir {
		dir := strings.TrimPrefix(r.Dir, opts.RomDir)
		vidPath = filepath.Join(opts.VidDir, dir)
	} else {
		vidPath = opts.VidDir
	}
	iName := r.BaseName + opts.VidSuffix
	return filepath.Join(vidPath, iName)
}

// getMarqPath gets the path to use for marquees.
func getMarqPath(r *ROM, opts *XMLOpts) string {
	var marqPath string
	if opts.NestImgDir {
		dir := strings.TrimPrefix(r.Dir, opts.RomDir)
		marqPath = filepath.Join(opts.MarqDir, dir)
	} else {
		marqPath = opts.MarqDir
	}
	iName := fmt.Sprintf("%s%s.%s", r.BaseName, opts.MarqSuffix, opts.MarqFormat)
	return filepath.Join(marqPath, iName)
}

// fixPaths fixes relative file paths to include the leading './'.
func fixPath(xmlDir, localDir, p string) string {
	s := strings.TrimPrefix(filepath.Clean(p), filepath.Clean(localDir))
	s = path.Clean(path.Join(xmlDir, filepath.ToSlash(s)))
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "~") {
		return s
	}
	return fmt.Sprintf("./%s", s)
}

var imgDirs = make(map[string]bool)

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

func getVideo(ctx context.Context, dsVid ds.Video, p string) error {
	dir := filepath.Dir(p)
	if !imgDirs[dir] {
		err := mkDir(dir)
		if err != nil {
			return err
		}
		imgDirs[dir] = true
	}
	return dsVid.Save(ctx, p)
}

// getImage gets the image, resizes it and saves it to specified path.
func getImage(ctx context.Context, dsImg ds.Image, p string, w uint, h uint) error {
	dir := filepath.Dir(p)
	if !imgDirs[dir] {
		err := mkDir(dir)
		if err != nil {
			return err
		}
		imgDirs[dir] = true
	}
	<-lock
	defer func() {
		lock <- struct{}{}
	}()
	return dsImg.Save(ctx, p, w, h)
}

func exists(s string) bool {
	fi, err := os.Stat(s)
	return err == nil && fi.Size() > 0
}

// fileExists checks if an image exists with either format.
func fileExists(p string, ext ...string) (string, bool) {
	if exists(p) {
		return p, true
	}
	e := filepath.Ext(p)
	for _, x := range ext {
		if e == x {
			continue
		}
		op := p[:len(p)-len(e)] + x
		if exists(op) {
			return op, true
		}
	}
	return p, false
}

// convertVideo transcodes a video using ffmpeg.
func convertVideo(p string) error {
	vidExt := filepath.Ext(p)
	baseFile := p[:len(p)-len(vidExt)]
	outputFile := baseFile + "-converting" + vidExt
	// Hardcoded command for now, clean this up once we offer more
	// conversion options.
	cmd := exec.Command("ffmpeg", "-i", p,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-vf", "scale=w=320:h=-2",
		"-c:a", "aac",
		"-b:a", "80k",
		outputFile)
	if err := cmd.Run(); err != nil {
		return err
	}
	return os.Rename(outputFile, p)
}

// XML creates the XML for the ROM after the Game has been populates.
func (r *ROM) XML(ctx context.Context, opts *XMLOpts) (*GameXML, error) {
	gxml := &GameXML{
		Path:        fixPath(opts.RomXMLDir, opts.RomDir, r.Path),
		ID:          r.Game.ID,
		GameTitle:   r.Game.GameTitle,
		Overview:    r.Game.Overview,
		Rating:      r.Game.Rating,
		ReleaseDate: r.Game.ReleaseDate,
		Developer:   r.Game.Developer,
		Publisher:   r.Game.Publisher,
		Genre:       r.Game.Genre,
		Source:      r.Game.Source,
		CloneOf:     r.Game.CloneOf,
	}
	if r.Game.Players > 0 {
		gxml.Players = strconv.FormatInt(r.Game.Players, 10)
	}
	imgPath := getImgPath(r, opts)
	imgPath, exists := fileExists(imgPath, imgExts...)
	if exists {
		gxml.Image = fixPath(opts.ImgXMLDir, opts.ImgDir, imgPath)
	}
	if !exists && !opts.NoDownload {
		for _, it := range opts.ImgPriority {
			var dsImg ds.Image
			if opts.ThumbOnly {
				dsImg = r.Game.Thumbs[it]
			} else {
				dsImg = r.Game.Images[it]
			}
			if dsImg == nil {
				continue
			}
			if err := getImage(ctx, dsImg, imgPath, opts.ImgWidth, opts.ImgHeight); err != nil {
				if err == ds.ErrImgNotFound {
					continue
				}
				return nil, err
			}
			gxml.Image = fixPath(opts.ImgXMLDir, opts.ImgDir, imgPath)
			break
		}
	}
	vidPath := getVidPath(r, opts)
	newPath, exists := fileExists(vidPath+".mp4", vidExts...)
	if exists {
		gxml.Video = fixPath(opts.VidXMLDir, opts.VidDir, newPath)
	}
	if !exists && opts.DownloadVid {
		for _, vt := range opts.VidPriority {
			dsVid := r.Game.Videos[vt]
			if dsVid == nil {
				continue
			}
			newPath = vidPath + dsVid.Ext()
			if err := getVideo(ctx, dsVid, newPath); err != nil {
				if err == ds.ErrImgNotFound {
					continue
				}
				return nil, err
			}
			if opts.VidConvert {
				if err := convertVideo(newPath); err != nil {
					return nil, err
				}
			}

			gxml.Video = fixPath(opts.VidXMLDir, opts.VidDir, newPath)

		}
	}
	imgPath = getMarqPath(r, opts)
	imgPath, exists = fileExists(imgPath, imgExts...)
	if exists {
		gxml.Marquee = fixPath(opts.MarqXMLDir, opts.MarqDir, imgPath)
	}
	if !exists && opts.DownloadMarq {
		if dsImg := r.Game.Images[ds.ImgMarquee]; dsImg != nil {
			if err := getImage(ctx, dsImg, imgPath, opts.ImgWidth, opts.ImgHeight); err != nil && err != ds.ErrImgNotFound {
				return nil, err
			}
			gxml.Marquee = fixPath(opts.MarqXMLDir, opts.MarqDir, imgPath)
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
	Marquee     string   `xml:"marquee,omitempty"`
	Video       string   `xml:"video,omitempty"`
	CloneOf     string   `xml:"cloneof,omitempty"`
	Hidden      string   `xml:"hidden,omitempty"`
	KidGame     string   `xml:"kidgame,omitempty"`
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
