package ds

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sselph/scraper/gdb"
)

// Daphne is a data source using GDB for Daphne games.
type Daphne struct {
	HM *HashMap
}

// GetID implements DS.
func (d *Daphne) GetID(p string) (string, error) {
	if filepath.Ext(p) != ".daphne" {
		return "", ErrNotFound
	}
	gameID := filepath.Base(p)
	switch {
	case strings.HasPrefix(gameID, "lair2_"):
		gameID = "lair2_*.daphne"
	case strings.HasPrefix(gameID, "sdq"):
		gameID = "sdq*.daphne"
	case strings.HasPrefix(gameID, "tq"):
		gameID = "tq*.daphne"
	}
	id, ok := d.HM.GetID(gameID)
	if !ok {
		return "", ErrNotFound
	}
	return id, nil
}

// GetName implements DS.
func (d *Daphne) GetName(p string) string {
	gameID := filepath.Base(p)
	n, ok := d.HM.GetName(gameID)
	if !ok {
		return ""
	}
	return n
}

// GetGame implements DS.
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
	ret := ParseGDBGame(game, resp.ImageURL)
	ret.ID = id
	return ret, nil
}
