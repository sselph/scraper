package ds

import (
	"fmt"
	"path/filepath"

	"github.com/danmcgoo/scraper/gdb"
)

// NeoGeo is a data source using GDB for Daphne games.
type NeoGeo struct {
	HM *HashMap
}

// GetID implements DS.
func (n *NeoGeo) GetID(p string) (string, error) {
	if filepath.Ext(p) == ".7z" {
		p = p[:len(p)-3] + ".zip"
	}
	if filepath.Ext(p) != ".zip" {
		return "", ErrNotFound
	}
	gameID := filepath.Base(p)
	id, ok := n.HM.GetID(gameID)
	if !ok {
		return "", ErrNotFound
	}
	return id, nil
}

// GetName implements DS.
func (n *NeoGeo) GetName(p string) string {
	gameID := filepath.Base(p)
	name, ok := n.HM.GetName(gameID)
	if !ok {
		return ""
	}
	return name
}

// GetGame implements DS.
func (n *NeoGeo) GetGame(id string) (*Game, error) {
	req := gdb.GGReq{ID: id}
	resp, err := gdb.GetGame(req)
	if err != nil {
		return nil, err
	}
	if len(resp.Game) == 0 {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}
	game := resp.Game[0]
	ret := ParseGDBGame(game, resp.ImageURL)
	ret.ID = id
	ret.Images[ImgTitle] = ret.Images[ImgBoxart]
	ret.Thumbs[ImgTitle] = ret.Thumbs[ImgBoxart]
	ret.Images[ImgMarquee] = ret.Images[ImgLogo]
	ret.Thumbs[ImgMarquee] = ret.Thumbs[ImgLogo]
	return ret, nil
}
