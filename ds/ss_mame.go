package ds

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sselph/scraper/ss"
)

// SSMAME is the source for ScreenScraper
type SSMAME struct {
	Dev    ss.DevInfo
	User   ss.UserInfo
	Lang   []string
	Region []string
	Width  int
	Height int
	Limit  chan struct{}
}

// GetName implements DS
func (s *SSMAME) GetName(p string) string {
	return ""
}

// GetGame implements DS
func (s *SSMAME) GetGame(path string) (*Game, error) {
	if s.Limit != nil {
		<-s.Limit
		defer func() {
			s.Limit <- struct{}{}
		}()
	}
	req := ss.GameInfoReq{Name: filepath.Base(path)}
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
	if game.Media.ScreenMarquee != "" {
		ret.Images[ImgTitle] = HTTPImageSS{game.Media.ScreenMarquee, s.Limit}
		ret.Thumbs[ImgTitle] = HTTPImageSS{game.Media.ScreenMarquee, s.Limit}
	}
	if game.Media.Marquee != "" {
		ret.Images[ImgMarquee] = HTTPImageSS{game.Media.Marquee, s.Limit}
		ret.Thumbs[ImgMarquee] = HTTPImageSS{game.Media.Marquee, s.Limit}
	}
	if imgURL, ok := game.Media.Box2D(regions); ok {
		ret.Images[ImgBoxart] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgBoxart] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.Media.Box3D(regions); ok {
		ret.Images[ImgBoxart3D] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgBoxart3D] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.Media.Flyer(regions); ok {
		ret.Images[ImgFlyer] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgFlyer] = HTTPImageSS{imgURL, s.Limit}
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
	if ret.Overview == "" && ret.ReleaseDate == "" && ret.Developer == "" && ret.Publisher == "" && ret.Images[ImgBoxart] == nil && ret.Images[ImgScreen] == nil {
		return nil, ErrNotFound
	}
	return ret, nil
}
