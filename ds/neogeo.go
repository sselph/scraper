package ds

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sselph/scraper/gdb"
)

// NeoGeo is a data source using GDB for Daphne games.
type NeoGeo struct {
	HM     *HashMap
	APIKey string
}

// getID gets the ID from the path.
func (n *NeoGeo) getID(p string) (string, error) {
	if filepath.Ext(p) == ".7z" {
		p = p[:len(p)-3] + ".zip"
	}
	if filepath.Ext(p) != ".zip" {
		return "", ErrNotFound
	}
	gameID := filepath.Base(p)
	id, ok := n.HM.ID(gameID)
	if !ok {
		return "", ErrNotFound
	}
	return id, nil
}

// GetName implements DS.
func (n *NeoGeo) GetName(p string) string {
	gameID := filepath.Base(p)
	name, ok := n.HM.Name(gameID)
	if !ok {
		return ""
	}
	return name
}

// GetGame implements DS.
func (n *NeoGeo) GetGame(ctx context.Context, id string) (*Game, error) {
	resp, err := gdb.GetGame(ctx, n.APIKey, id)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}

	result := ParseGDBGame(*resp)
	result.Images[ImgTitle] = result.Images[ImgBoxart]
	result.Thumbs[ImgTitle] = result.Thumbs[ImgBoxart]
	result.Images[ImgMarquee] = result.Images[ImgLogo]
	result.Thumbs[ImgMarquee] = result.Thumbs[ImgLogo]
	return result, nil
}

func (source *NeoGeo) GetNames(ps []string) []string {
	results := make([]string, 0, len(ps))

	for _, p := range ps {
		results = append(results, source.GetName(p))
	}

	return results
}

func (source *NeoGeo) GetGames(ctx context.Context, ids []string) []GameResult {
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

func (source *NeoGeo) GetIds(ps []string) []IDResult {
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
