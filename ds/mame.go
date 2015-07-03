package ds

import (
	"path/filepath"

	"github.com/sselph/scraper/mamedb"
)

type MAME struct{}

func (m *MAME) GetID(p string) (string, error) {
	b := filepath.Base(p)
	id := b[:len(b)-len(filepath.Ext(b))]
	return id, nil
}

func (m *MAME) GetName(p string) string {
	return ""
}

func (m *MAME) GetGame(id string) (*Game, error) {
	g, err := mamedb.GetGame(id)
	if err != nil {
		return nil, err
	}
	game := NewGame()
	game.ID = g.ID
	game.GameTitle = g.Name
	game.ReleaseDate = g.Date
	game.Developer = g.Developer
	game.Genre = g.Genre
	game.Source = g.Source
	game.Players = g.Players
	game.Rating = g.Rating / 10.0
	if g.Title != "" {
		game.Images[IMG_TITLE] = g.Title
		game.Thumbs[IMG_TITLE] = g.Title
	}
	if g.Snap != "" {
		game.Images[IMG_SCREEN] = g.Snap
		game.Thumbs[IMG_SCREEN] = g.Snap
	}
	if g.Marquee != "" {
		game.Images[IMG_MARQUEE] = g.Marquee
		game.Thumbs[IMG_MARQUEE] = g.Marquee
	}
	if g.Cabinet != "" {
		game.Images[IMG_CABINET] = g.Cabinet
		game.Thumbs[IMG_CABINET] = g.Cabinet
	}
	return game, nil
}
