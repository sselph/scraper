package ds

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
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

type HTTPVideoSS struct {
	URL   string
	E     string
	Limit chan struct{}
}

func (v HTTPVideoSS) Save(ctx context.Context, p string) error {
	if v.Limit != nil {
		v.Limit <- struct{}{}
		defer func() {
			<-v.Limit
		}()
	}
	req, err := http.NewRequest("GET", v.URL, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrImgNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%v from $s", resp.StatusCode, v.URL)
	}
	defer resp.Body.Close()
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func (v HTTPVideoSS) Ext() string {
	return v.E
}

type HTTPImageSS struct {
	URL   string
	Limit chan struct{}
}

func (i HTTPImageSS) fetch(ctx context.Context, width, height uint) (rc io.ReadCloser, err error) {
	u := ssImgURL(i.URL, int(width), int(height))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if uerr, ok := err.(*url.Error); ok {
			uerr.URL = ss.SanitizeURL(uerr.URL)
			return nil, uerr
		}
		return nil, err
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrImgNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v from %s", resp.StatusCode, ss.SanitizeURL(u))
	}
	if resp.Header.Get("Content-Type") != "image/png" {
		b, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s from %s", string(b), ss.SanitizeURL(u))
	}
	return resp.Body, nil
}

func (i HTTPImageSS) Get(ctx context.Context, width, height uint) (image.Image, error) {
	if i.Limit != nil {
		i.Limit <- struct{}{}
		defer func() {
			<-i.Limit
		}()
	}
	b, err := i.fetch(ctx, width, height)
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

func (i HTTPImageSS) Save(ctx context.Context, p string, width, height uint) error {
	if i.Limit != nil {
		i.Limit <- struct{}{}
		defer func() {
			<-i.Limit
		}()
	}
	b, err := i.fetch(ctx, width, height)
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
func (s *SS) GetGame(ctx context.Context, path string) (*Game, error) {
	if s.Limit != nil {
		s.Limit <- struct{}{}
		defer func() {
			<-s.Limit
		}()
	}
	id, err := s.getID(path)
	if err != nil {
		return nil, err
	}
	// Empty File, SS still returns a result.
	if id == "da39a3ee5e6b4b0d3255bfef95601890afd80709" {
		return nil, ErrNotFound
	}
	req := ss.GameInfoReq{SHA1: id}
	resp, err := ss.GameInfo(ctx, s.Dev, s.User, req)
	if err != nil {
		if err == ss.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	game := resp.Response.Game
	var regions []string
	rom, ok := game.ROM(req)
	if !ok {
		return nil, ErrNotFound
	}
	for _, r := range rom.Regions() {
		regions = append(regions, r)
	}
	regions = append(regions, s.Region...)

	ret := NewGame()
	var screen, box, cart, wheel Image
	screen = addImageToGame(ret, game, ss.Screenshot, ImgScreen, regions, s)
	addImageToGame(ret, game, ss.Box2D, ImgBoxart, regions, s)
	box = addImageToGame(ret, game, ss.Box3D, ImgBoxart3D, regions, s)
	wheel = addImageToGame(ret, game, ss.Wheel, ImgMarquee, regions, s)
	cart = addImageToGame(ret, game, ss.Support2D, ImgCart, regions, s)
	addImageToGame(ret, game, ss.SupportLabel, ImgCartLabel, regions, s)
	if vidURL, format, ok := game.MediaWithFormat(ss.Video, regions); ok {
		ret.Videos[VidStandard] = HTTPVideoSS{vidURL, "." + format, s.Limit}
	}
	ret.Images[ImgMix3] = MixImage{StandardThree(screen, box, wheel)}
	ret.Thumbs[ImgMix3] = MixImage{StandardThree(screen, box, wheel)}
	ret.Images[ImgMix4] = MixImage{StandardFour(screen, box, cart, wheel)}
	ret.Thumbs[ImgMix4] = MixImage{StandardFour(screen, box, cart, wheel)}
	ret.ID = game.ID
	ret.Source = "screenscraper.fr"
	ret.GameTitle, _ = game.Name(s.Region)
	ret.Overview, _ = game.Desc(s.Lang)
	game.Rating.Text = strings.TrimSuffix(game.Rating.Text, "/20")
	if r, err := strconv.ParseFloat(game.Rating.Text, 64); err == nil {
		ret.Rating = r / 20.0
	}
	ret.Developer = game.Developer.Text
	ret.Publisher = game.Publisher.Text
	ret.Genre, _ = game.Genre(s.Lang)
	if r, ok := game.Date(s.Region); ok {
		ret.ReleaseDate = ssXMLDate(r)
	}
	if strings.ContainsRune(game.Players.Text, '-') {
		x := strings.Split(game.Players.Text, "-")
		game.Players.Text = x[len(x)-1]
	}
	p, err := strconv.ParseInt(strings.TrimRight(game.Players.Text, "+"), 10, 32)
	if err == nil {
		ret.Players = p
	}
	if ret.Overview == "" && ret.Images[ImgBoxart] == nil && ret.Images[ImgScreen] == nil {
		return nil, ErrNotFound
	}
	return ret, nil
}

func addImageToGame(ret *Game, game ss.Game, mediaType ss.MediaType, imgType ImgType, regions []string, s *SS) HTTPImageSS {
	var gameImage HTTPImageSS
	if imgURL, ok := game.Media(mediaType, regions); ok {
		gameImage = HTTPImageSS{imgURL, s.Limit}
		ret.Images[imgType] = gameImage
		ret.Thumbs[imgType] = gameImage
	}
	return gameImage
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
