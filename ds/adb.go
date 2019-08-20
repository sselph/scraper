package ds

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/sselph/scraper/adb"
)

var bioRE = regexp.MustCompile(`- (CAST|CONTRIBUTE|PORTS|SCORING|SERIES|STAFF|TECHNICAL|TRIVIA|UPDATES) -`)

// ADB is a Data Source using arcadeitalia and arcade-history.
type ADB struct {
	Limit chan struct{}
}

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
func (a *ADB) GetGame(ctx context.Context, p string) (*Game, error) {
	if a.Limit != nil {
		a.Limit <- struct{}{}
		defer func() {
			<-a.Limit
		}()
	}
	id, err := a.getID(p)
	if err != nil {
		return nil, err
	}
	r, err := adb.GetGame(ctx, id)
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
	game.CloneOf = g.CloneOf
	game.Source = adb.Source
	game.Players = g.Players
	game.Rating = float64(g.Rating) / 100
	if g.History != "" {
		parts := bioRE.Split(g.History, 2)
		game.Overview = fmt.Sprintf("%s\n\n%s", parts[0], g.CopyRightShort)
	}
	if g.Title != "" {
		game.Images[ImgTitle] = HTTPImage{URL: g.Title, Limit: a.Limit}
		game.Thumbs[ImgTitle] = HTTPImage{URL: g.Title, Limit: a.Limit}
	}
	if g.Snap != "" {
		game.Images[ImgScreen] = HTTPImage{URL: g.Snap, Limit: a.Limit}
		game.Thumbs[ImgScreen] = HTTPImage{URL: g.Snap, Limit: a.Limit}
	}
	if g.Marquee != "" {
		game.Images[ImgMarquee] = HTTPImage{URL: g.Marquee, Limit: a.Limit}
		game.Thumbs[ImgMarquee] = HTTPImage{URL: g.Marquee, Limit: a.Limit}
	}
	if g.Cabinet != "" {
		game.Images[ImgCabinet] = HTTPImage{URL: g.Cabinet, Limit: a.Limit}
		game.Thumbs[ImgCabinet] = HTTPImage{URL: g.Cabinet, Limit: a.Limit}
	}
	if g.Flyer != "" {
		game.Images[ImgFlyer] = HTTPImage{URL: g.Flyer, Limit: a.Limit}
		game.Thumbs[ImgFlyer] = HTTPImage{URL: g.Flyer, Limit: a.Limit}
	}
	if g.Video != "" {
		game.Videos[VidStandard] = HTTPVideo{URL: g.Video, E: ".mp4", Limit: a.Limit}
	}
	return game, nil
}

func (source ADB) GetNames(ps []string) []string {
	results := make([]string, 0, len(ps))

	for _, p := range ps {
		results = append(results, source.GetName(p))
	}

	return results
}

func (source ADB) GetGames(ctx context.Context, ps []string) []GameResult {
	results := make([]GameResult, 0, len(ps))

	for _, p := range ps {
		game, err := source.GetGame(ctx, p)
		results = append(results, GameResult{
			Game:  game,
			Error: err,
		})
	}

	return results
}
