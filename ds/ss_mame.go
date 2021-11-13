package ds

import (
	"context"
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
	addImageToMameGame(ret, game, ss.Screenshot, ImgScreen, regions, s)
	addImageToMameGame(ret, game, ss.ScreenMarquee, ImgTitle, regions, s)
	addImageToMameGame(ret, game, ss.Marquee, ImgMarquee, regions, s)
	addImageToMameGame(ret, game, ss.Box2D, ImgBoxart, regions, s)
	addImageToMameGame(ret, game, ss.Box3D, ImgBoxart3D, regions, s)
	addImageToMameGame(ret, game, ss.Flyer, ImgFlyer, regions, s)
	if vidURL, format, ok := game.MediaWithFormat(ss.Video, regions); ok {
		ret.Videos[VidStandard] = HTTPVideoSS{vidURL, "." + format, s.Limit}
	}
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
	if ret.Overview == "" && ret.ReleaseDate == "" && ret.Developer == "" && ret.Publisher == "" && ret.Images[ImgBoxart] == nil && ret.Images[ImgScreen] == nil {
		return nil, ErrNotFound
	}
	return ret, nil
}

func addImageToMameGame(ret *Game, game ss.Game, mediaType ss.MediaType, imgType ImgType, regions []string, s *SSMAME) HTTPImageSS {
	var gameImage HTTPImageSS
	if imgURL, ok := game.Media(mediaType, regions); ok {
		gameImage = HTTPImageSS{imgURL, s.Limit}
		ret.Images[imgType] = gameImage
		ret.Thumbs[imgType] = gameImage
	}
	return gameImage
}
