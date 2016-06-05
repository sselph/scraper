package ds

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sselph/scraper/gdb"
)

// Daphne is a data source using GDB for Daphne games.
type Daphne struct {
	HM *HashMap
}

func (d *Daphne) GetID(p string) (string, error) {
	if filepath.Ext(p) != ".daphne" {
		return "", NotFoundErr
	}
	gameID := filepath.Base(p)
	id, ok := d.HM.GetID(gameID)
	if !ok {
		return "", NotFoundErr
	}
	return id, nil
}

func (d *Daphne) GetName(p string) string {
	gameID := filepath.Base(p)
	n, ok := d.HM.GetName(gameID)
	if !ok {
		return ""
	}
	return n
}

func (d *Daphne) GetGame(id string) (*Game, error) {
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
