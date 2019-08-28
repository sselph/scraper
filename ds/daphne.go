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
func (d *Daphne) GetGame(ctx context.Context, id string) (*Game, error) {
	gameResult := gdb.GetGames(ctx, d.APIKey, []string{id})[0]
	resp := gameResult.Game
	err := gameResult.Error
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}

	result := ParseGDBGame(*resp)
	return result, nil
}

func (source *Daphne) GetNames(ps []string) []string {
	results := make([]string, 0, len(ps))

	for _, p := range ps {
		results = append(results, source.GetName(p))
	}

	return results
}

func (source *Daphne) GetGames(ctx context.Context, ids []string) []GameResult {
	results := make([]GameResult, 0, len(ids))

	for _, id := range ids {
		game, err := source.GetGame(ctx, id)
		results = append(results, GameResult{
			Game:  game,
			Error: err,
		})
	}

	return results
}

func (source *Daphne) GetIds(ps []string) []IDResult {
	results := make([]IDResult, 0, len(ps))

	for _, p := range ps {
		id, err := source.getID(p)
		results = append(results, IDResult{
			ID:    id,
			Error: err,
		})
	}

	return results
}
