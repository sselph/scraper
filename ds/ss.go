package ds

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sselph/scraper/ss"
)

func ssXMLDate(d string) string {
	switch len(d) {
	case 10:
		t, _ := time.Parse("2006-01-02", d)
		return t.Format("20060102T000000")
	case 4:
		return fmt.Sprintf("%s0101T000000", d)
	}
	return ""
}

// SS is the source for ScreenScraper
type SS struct {
	HM     *HashMap
	Hasher *Hasher
	Dev    ss.DevInfo
	User   ss.UserInfo
	Lang   []string
	Region []string
	Width  int
	Height int
	Limit  chan struct{}
}

type HTTPImageSS struct {
	URL   string
	Limit chan struct{}
}

func (i HTTPImageSS) fetch(width, height uint) (io.ReadCloser, error) {
	u := ssImgURL(i.URL, int(width), int(height))
	resp, err := http.Get(u)
	if err != nil {
		if uerr, ok := err.(*url.Error); ok {
			uerr.URL = ss.SanitizeURL(uerr.URL)
			return nil, uerr
		}
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrImgNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v from $s", resp.StatusCode, ss.SanitizeURL(u))
	}
	return resp.Body, nil
}

func (i HTTPImageSS) Get(width, height uint) (image.Image, error) {
	if i.Limit != nil {
		<-i.Limit
		defer func() {
			i.Limit <- struct{}{}
		}()
	}
	b, err := i.fetch(width, height)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	img, _, err := image.Decode(b)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (i HTTPImageSS) Save(p string, width, height uint) error {
	if i.Limit != nil {
		<-i.Limit
		defer func() {
			i.Limit <- struct{}{}
		}()
	}
	b, err := i.fetch(width, height)
	if err != nil {
		return err
	}
	defer b.Close()
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()
	e := filepath.Ext(p)
	switch e {
	case ".jpg":
		img, _, err := image.Decode(b)
		if err != nil {
			return nil
		}
		return jpeg.Encode(out, img, nil)
	case ".png":
		_, err := io.Copy(out, b)
		return err
	default:
		return fmt.Errorf("Invalid image type.")
	}
	return nil
}

// getID gets the ID from the path.
func (s *SS) getID(p string) (string, error) {
	return s.Hasher.Hash(p)
}

// GetName implements DS
func (s *SS) GetName(p string) string {
	h, err := s.Hasher.Hash(p)
	if err != nil {
		return ""
	}
	name, ok := s.HM.Name(h)
	if !ok {
		return ""
	}
	return name
}

// GetGame implements DS
func (s *SS) GetGame(path string) (*Game, error) {
	if s.Limit != nil {
		<-s.Limit
		defer func() {
			s.Limit <- struct{}{}
		}()
	}
	id, err := s.getID(path)
	if err != nil {
		return nil, err
	}
	req := ss.GameInfoReq{SHA1: id}
	resp, err := ss.GameInfo(s.Dev, s.User, req)
	if err != nil {
		if err == ss.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	game := resp.Response.Game
	var regions []string
	for _, r := range game.ROM(req).Regions() {
		regions = append(regions, r)
	}
	regions = append(regions, s.Region...)

	ret := NewGame()
	if game.Media.Screenshot != "" {
		ret.Images[ImgScreen] = HTTPImageSS{game.Media.Screenshot, s.Limit}
		ret.Thumbs[ImgScreen] = HTTPImageSS{game.Media.Screenshot, s.Limit}
	}
	if imgURL, ok := game.Media.Box2D(regions); ok {
		ret.Images[ImgBoxart] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgBoxart] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.Media.Box3D(regions); ok {
		ret.Images[ImgBoxart3D] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgBoxart3D] = HTTPImageSS{imgURL, s.Limit}
	}
	ret.ID = game.ID
	ret.Source = "screenscraper.fr"
	ret.GameTitle = game.Name
	ret.Overview, _ = game.Desc(s.Lang)
	game.Rating = strings.TrimSuffix(game.Rating, "/20")
	if r, err := strconv.ParseFloat(game.Rating, 64); err == nil {
		ret.Rating = r / 20.0
	}
	ret.Developer = game.Developer
	ret.Publisher = game.Publisher
	ret.Genre, _ = game.Genre(s.Lang)
	if r, ok := game.Date(s.Region); ok {
		ret.ReleaseDate = ssXMLDate(r)
	}
	if strings.ContainsRune(game.Players, '-') {
		x := strings.Split(game.Players, "-")
		game.Players = x[len(x)-1]
	}
	p, err := strconv.ParseInt(strings.TrimRight(game.Players, "+"), 10, 32)
	if err == nil {
		ret.Players = p
	}
	if ret.Overview == "" && ret.Images[ImgBoxart] == nil && ret.Images[ImgScreen] == nil) {
		return nil, ErrNotFound
	}
	return ret, nil
}

// ssImgURL parses the URL and adds the maxwidth.
func ssImgURL(img string, width int, height int) string {
	if width <= 0 && height <= 0 {
		return img
	}
	u, err := url.Parse(img)
	if err != nil {
		return img
	}
	v := u.Query()
	if width > 0 {
		v.Set("maxwidth", strconv.Itoa(width))
	}
	if height > 0 {
		v.Set("maxheight", strconv.Itoa(height))
	}
	u.RawQuery = v.Encode()
	return u.String()
}
