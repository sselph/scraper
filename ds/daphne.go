package ds

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sselph/scraper/gdb"
)

// Daphne is a data source using GDB for Daphne games.
type Daphne struct {
	HM     *HashMap
	APIKey string
}

// GetID implements DS.
func (d *Daphne) getID(p string) (string, error) {
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
	id, ok := d.HM.ID(gameID)
	if !ok {
		return "", ErrNotFound
	}
	return id, nil
}

// GetName implements DS.
func (d *Daphne) GetName(p string) string {
	gameID := filepath.Base(p)
	n, ok := d.HM.Name(gameID)
	if !ok {
		return ""
	}
	return n
}

// GetGame implements DS.
func (d *Daphne) GetGame(ctx context.Context, p string) (*Game, error) {
	id, err := d.getID(p)
	if err != nil {
		return nil, err
	}
	req := gdb.GGReq{ID: id}
	resp, err := gdb.GetGame(ctx, d.APIKey, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}

	result := ParseGDBGame(*resp)
	return result, nil
}
