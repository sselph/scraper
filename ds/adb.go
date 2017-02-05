package ds

import (
	"fmt"
	"regexp"
	"strconv"
	"path/filepath"

	"github.com/sselph/scraper/adb"
)

var bioRE = regexp.MustCompile(`- (CAST|CONTRIBUTE|PORTS|SCORING|SERIES|STAFF|TECHNICAL|TRIVIA|UPDATES) -`)

// ADB is a Data Source using arcadeitalia and arcade-history.
type ADB struct {}

// getID gets the ID for the game..
func (a *ADB) getID(p string) (string, error) {
	b := filepath.Base(p)
	id := b[:len(b)-len(filepath.Ext(b))]
	return id, nil
}

// GetName implements DS.
func (a *ADB) GetName(p string) string {
	return ""
}

// GetGame implements DS.
func (a *ADB) GetGame(p string) (*Game, error) {
	id, err := a.getID(p)
	if err != nil {
		return nil, err
	}
	r, err := adb.GetGame(id)
	if err != nil {
		return nil, err
	}
	if len(r.Results) == 0 {
		return nil, ErrNotFound
	}
	g := r.Results[0]
	game := NewGame()
	game.ID = g.ID
	game.GameTitle = g.Name
	game.ReleaseDate = g.Year
	game.Developer = g.Manufacturer
	game.Genre = g.Genre
	game.Source = adb.Source
	if p, err := strconv.ParseInt(g.Players, 10, 32); err == nil {
		game.Players = p
	}
	if g.History != "" {
		parts := bioRE.Split(g.History, 2)
		game.Overview = fmt.Sprintf("%s\n\n%s", parts[0], g.CopyRightShort)
	}
	if g.Title != "" {
		game.Images[ImgTitle] = HTTPImage{g.Title}
		game.Thumbs[ImgTitle] = HTTPImage{g.Title}
	}
	if g.Snap != "" {
		game.Images[ImgScreen] = HTTPImage{g.Snap}
		game.Thumbs[ImgScreen] = HTTPImage{g.Snap}
	}
	if g.Marquee != "" {
		game.Images[ImgMarquee] = HTTPImage{g.Marquee}
		game.Thumbs[ImgMarquee] = HTTPImage{g.Marquee}
	}
	if g.Cabinet != "" {
		game.Images[ImgCabinet] = HTTPImage{g.Cabinet}
		game.Thumbs[ImgCabinet] = HTTPImage{g.Cabinet}
	}
	return game, nil
}
