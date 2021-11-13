package ds

import (
	"context"
	"net/url"
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
func (s *SSMAME) GetGame(ctx context.Context, path string) (*Game, error) {
	if s.Limit != nil {
		s.Limit <- struct{}{}
		defer func() {
			<-s.Limit
		}()
	}
	req := ss.GameInfoReq{Name: filepath.Base(path)}
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
	if imgURL, ok := game.Screenshot(regions); ok {
		ret.Images[ImgScreen] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgScreen] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.ScreenMarquee(regions); ok {
		ret.Images[ImgTitle] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgTitle] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.Marquee(regions); ok {
		ret.Images[ImgMarquee] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgMarquee] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.Box2D(regions); ok {
		ret.Images[ImgBoxart] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgBoxart] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.Box3D(regions); ok {
		ret.Images[ImgBoxart3D] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgBoxart3D] = HTTPImageSS{imgURL, s.Limit}
	}
	if imgURL, ok := game.Flyer(regions); ok {
		ret.Images[ImgFlyer] = HTTPImageSS{imgURL, s.Limit}
		ret.Thumbs[ImgFlyer] = HTTPImageSS{imgURL, s.Limit}
	}
	if vidURL, ok := game.Video(regions); ok {
		if u, err := url.Parse(vidURL); err == nil {
			ext := u.Query().Get("mediaformat")
			ret.Videos[VidStandard] = HTTPVideoSS{vidURL, "." + ext, s.Limit}
		}
	}
	ret.ID = game.ID
	ret.Source = "screenscraper.fr"
	ret.GameTitle = game.Name
	ret.Overview, _ = game.Desc(s.Lang)
	game.Rating = strings.TrimSuffix(game.Rating, "/20")
	if r, err := strconv.ParseFloat(game.Rating, 64); err == nil {
		ret.Rating = r / 20.0
	}
	ret.Developer = game.Developer.Text
	ret.Publisher = game.Publisher.Text
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
