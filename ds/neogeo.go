package ds

import (
	"fmt"
	"path/filepath"

	"github.com/sselph/scraper/gdb"
)

// NeoGeo is a data source using GDB for Daphne games.
type NeoGeo struct {
	HM *HashMap
}

func (n *NeoGeo) GetID(p string) (string, error) {
	if filepath.Ext(p) == ".7z" {
		p = p[:len(p)-3] + ".zip"
	}
	if filepath.Ext(p) != ".zip" {
		return "", NotFoundErr
	}
	gameID := filepath.Base(p)
	id, ok := n.HM.GetID(gameID)
	if !ok {
		return "", NotFoundErr
	}
	return id, nil
}

func (n *NeoGeo) GetName(p string) string {
	gameID := filepath.Base(p)
	name, ok := n.HM.GetName(gameID)
	if !ok {
		return ""
	}
	return name
}

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
	return ret, nil
}
