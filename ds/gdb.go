package ds

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sselph/scraper/gdb"
)

// GDB is a DataSource using thegamesdb.net
type GDB struct {
	HM     *HashMap
	Hasher *Hasher
}

// getFront gets the front boxart for a Game if it exists.
func getFront(g gdb.Game) *gdb.Image {
	for _, v := range g.BoxArt {
		if v.Side == "front" {
			return &v
		}
	}
	return nil
}

// toXMLDate converts a gdb date to the gamelist.xml date.
func toXMLDate(d string) string {
	switch len(d) {
	case 10:
		t, _ := time.Parse("01/02/2006", d)
		return t.Format("20060102T000000")
	case 4:
		return fmt.Sprintf("%s0101T000000", d)
	}
	return ""
}

func (g *GDB) Hash(p string) (string, error) {
	return g.Hasher.Hash(p)
}

func (g *GDB) GetID(p string) (string, error) {
	h, err := g.Hasher.Hash(p)
	if err != nil {
		return "", err
	}
	id, ok := g.HM.GetID(h)
	if !ok {
		return "", NotFoundErr
	}
	return id, nil
}

func (g *GDB) GetName(p string) string {
	h, err := g.Hasher.Hash(p)
	if err != nil {
		return ""
	}
	name, ok := g.HM.GetName(h)
	if !ok {
		return ""
	}
	return name
}

func (g *GDB) GetGame(id string) (*Game, error) {
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
