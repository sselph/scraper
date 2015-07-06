package ds

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sselph/scraper/gdb"
)

// ScummVM is a data source using GDB for ScummVM games.
type ScummVM struct {
	HM *HashMap
}

func (s *ScummVM) GetID(p string) (string, error) {
	if filepath.Ext(p) != ".svm" {
		return "", NotFoundErr
	}
	b := filepath.Base(p)
	svm := b[:len(b)-len(filepath.Ext(b))]
	gameID := strings.Split(svm, "-")[0]
	id, ok := s.HM.GetID(gameID)
	if !ok {
		return "", NotFoundErr
	}
	return id, nil
}

func (s *ScummVM) GetName(p string) string {
	b := filepath.Base(p)
	svm := b[:len(b)-len(filepath.Ext(b))]
	gameID := strings.Split(svm, "-")[0]
	n, ok := s.HM.GetName(gameID)
	if !ok {
		return ""
	}
	return n
}

func (s *ScummVM) GetGame(id string) (*Game, error) {
	req := gdb.GGReq{ID: id}
	resp, err := gdb.GetGame(req)
	if err != nil {
		return nil, err
	}
	if len(resp.Game) == 0 {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}
	game := resp.Game[0]
	ret := NewGame()
	if len(game.Screenshot) != 0 {
		ret.Images[IMG_SCREEN] = resp.ImageURL + game.Screenshot[0].Original.URL
		ret.Thumbs[IMG_SCREEN] = resp.ImageURL + game.Screenshot[0].Thumb
	}
	front := getFront(game)
	if front != nil {
		ret.Images[IMG_BOXART] = resp.ImageURL + front.URL
		ret.Thumbs[IMG_BOXART] = resp.ImageURL + front.Thumb
	}
	if len(game.FanArt) != 0 {
		ret.Images[IMG_FANART] = resp.ImageURL + game.FanArt[0].Original.URL
		ret.Thumbs[IMG_FANART] = resp.ImageURL + game.FanArt[0].Thumb
	}
	if len(game.Banner) != 0 {
		ret.Images[IMG_BANNER] = resp.ImageURL + game.Banner[0].URL
		ret.Thumbs[IMG_BANNER] = resp.ImageURL + game.Banner[0].URL
	}
	if len(game.ClearLogo) != 0 {
		ret.Images[IMG_LOGO] = resp.ImageURL + game.ClearLogo[0].URL
		ret.Thumbs[IMG_LOGO] = resp.ImageURL + game.ClearLogo[0].URL
	}

	var genre string
	if len(game.Genres) >= 1 {
		genre = game.Genres[0]
	}
	ret.ID = id
	ret.GameTitle = game.GameTitle
	ret.Overview = game.Overview
	ret.Rating = game.Rating / 10.0
	ret.ReleaseDate = toXMLDate(game.ReleaseDate)
	ret.Developer = game.Developer
	ret.Publisher = game.Publisher
	ret.Genre = genre
	ret.Source = "theGamesDB.net"
	p, err := strconv.ParseInt(strings.TrimRight(game.Players, "+"), 10, 32)
	if err == nil {
		ret.Players = p
	}
	return ret, nil
}


