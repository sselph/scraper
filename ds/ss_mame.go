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
	Lang   []LangType
	Region []RegionType
	Width  int
	Height int
}

// GetName implements DS
func (s *SSMAME) GetName(p string) string {
	return ""
}

// GetGame implements DS
func (s *SSMAME) GetGame(path string) (*Game, error) {
	req := ss.GameInfoReq{Name: filepath.Base(path)}
	resp, err := ss.GameInfo(s.Dev, s.User, req)
	if err != nil {
		if err == ss.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	game := resp.Game

	region := RegionUnknown
	var regions []RegionType
	if region != RegionUnknown {
		regions = append([]RegionType{region}, s.Region...)
	} else {
		regions = s.Region
	}

	ret := NewGame()
	if game.Media.ScreenShot != "" {
		ret.Images[ImgScreen] = ssImgURL(game.Media.ScreenShot, s.Width, s.Height)
		ret.Thumbs[ImgScreen] = ssImgURL(game.Media.ScreenShot, s.Width, s.Height)
	}
	if game.Media.ScreenMarquee != "" {
		ret.Images[ImgTitle] = ssImgURL(game.Media.ScreenMarquee, s.Width, s.Height)
		ret.Thumbs[ImgTitle] = ssImgURL(game.Media.ScreenMarquee, s.Width, s.Height)
	}
	if game.Media.Marquee != "" {
		ret.Images[ImgMarquee] = ssImgURL(game.Media.Marquee, s.Width, s.Height)
		ret.Thumbs[ImgMarquee] = ssImgURL(game.Media.Marquee, s.Width, s.Height)
	}
	if imgURL := ssBoxURL(game.Media, regions, s.Width, s.Height); imgURL != "" {
		ret.Images[ImgBoxart] = imgURL
		ret.Thumbs[ImgBoxart] = imgURL
	}
	if imgURL := ss3DBoxURL(game.Media, regions, s.Width, s.Height); imgURL != "" {
		ret.Images[ImgBoxart3D] = imgURL
		ret.Thumbs[ImgBoxart3D] = imgURL
	}
	if imgURL := ssFlyerURL(game.Media, regions, s.Width, s.Height); imgURL != "" {
		ret.Images[ImgFlyer] = imgURL
		ret.Thumbs[ImgFlyer] = imgURL
	}
	ret.ID = strconv.Itoa(game.ID)
	ret.Source = "screenscraper.fr"
	ret.GameTitle = game.Names.Original
	ret.Overview = ssDesc(game.Desc, s.Lang)
	game.Rating = strings.TrimSuffix(game.Rating, "/20")
	if r, err := strconv.ParseFloat(game.Rating, 64); err == nil {
		ret.Rating = r / 20.0
	}
	ret.Developer = game.Developer
	ret.Publisher = game.Publisher
	ret.Genre = ssGenre(game.Genre, s.Lang)
	ret.ReleaseDate = ssDate(game.Dates, s.Region)
	if strings.ContainsRune(game.Players, '-') {
		x := strings.Split(game.Players, "-")
		game.Players = x[len(x)-1]
	}
	p, err := strconv.ParseInt(strings.TrimRight(game.Players, "+"), 10, 32)
	if err == nil {
		ret.Players = p
	}
	if ret.Overview == "" && ret.ReleaseDate == "" && ret.Developer == "" && ret.Publisher == "" && ret.Images[ImgBoxart] == "" && ret.Images[ImgScreen] == "" {
		return nil, ErrNotFound
	}
	return ret, nil
}
