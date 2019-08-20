package ds

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sselph/scraper/gdb"
)

// GDB is a DataSource using thegamesdb.net
type GDB struct {
	HM     *HashMap
	Hasher *Hasher
	APIKey string
}

// getFront gets the front boxart for a Game if it exists.
func getFront(images []gdb.ParsedGameImage) *gdb.ParsedGameImage {
	for _, image := range images {
		if image.Side == "front" {
			return &image
		}
	}
	return nil
}

// toXMLDate converts a gdb date to the gamelist.xml date.
func toXMLDate(d string) string {
	switch len(d) {
	case 10:
		t, _ := time.Parse("2006-01-02", d)
		return t.Format("20060102T000000")
	case 4:
		return fmt.Sprintf("%s0101T000000", d)
	}
	return ""
}

// Hash hashes a ROM.
func (g *GDB) Hash(p string) (string, error) {
	return g.Hasher.Hash(p)
}

// getID gets the ID from the path.
func (g *GDB) getID(p string) (string, error) {
	h, err := g.Hasher.Hash(p)
	if err != nil {
		return "", err
	}
	id, ok := g.HM.ID(h)
	if !ok {
		return "", ErrNotFound
	}
	return id, nil
}

type ImageTypeName string

func bucketImagesByType(images []gdb.ParsedGameImage) map[ImageTypeName][]gdb.ParsedGameImage {
	res := make(map[ImageTypeName][]gdb.ParsedGameImage)

	for _, image := range images {
		imageType := ImageTypeName(image.Type)
		res[imageType] = append(res[imageType], image)
	}

	return res
}

func parseImages(game gdb.ParsedGame) (map[ImgType]Image, map[ImgType]Image) {
	originals := make(map[ImgType]Image)
	thumbs := make(map[ImgType]Image)

	baseURLOriginal := game.ImageBaseUrls.Original
	baseURLThumb := game.ImageBaseUrls.Thumb

	gameImages := game.Images[strconv.Itoa(game.ID)]

	if len(gameImages) != 0 {
		bucketedImages := bucketImagesByType(gameImages)

		if imgs, ok := bucketedImages["screenshot"]; ok && len(imgs) > 0 {
			img := imgs[0]
			originals[ImgScreen] = HTTPImage{URL: baseURLOriginal + img.Filename}
			thumbs[ImgScreen] = HTTPImage{URL: baseURLThumb + img.Filename}
		}

		if imgs, ok := bucketedImages["boxart"]; ok && len(imgs) > 0 {
			if front := getFront(imgs); front != nil {
				originals[ImgBoxart] = HTTPImage{URL: baseURLOriginal + front.Filename}
				thumbs[ImgBoxart] = HTTPImage{URL: baseURLThumb + front.Filename}
			}
		}

		if imgs, ok := bucketedImages["fanart"]; ok && len(imgs) > 0 {
			img := imgs[0]
			originals[ImgFanart] = HTTPImage{URL: baseURLOriginal + img.Filename}
			thumbs[ImgFanart] = HTTPImage{URL: baseURLThumb + img.Filename}
		}

		if imgs, ok := bucketedImages["banner"]; ok && len(imgs) > 0 {
			img := imgs[0]
			originals[ImgBanner] = HTTPImage{URL: baseURLOriginal + img.Filename}
			thumbs[ImgBanner] = HTTPImage{URL: baseURLThumb + img.Filename}
		}

		if imgs, ok := bucketedImages["clearlogo"]; ok && len(imgs) > 0 {
			img := imgs[0]
			originals[ImgLogo] = HTTPImage{URL: baseURLOriginal + img.Filename}
			thumbs[ImgLogo] = HTTPImage{URL: baseURLThumb + img.Filename}
		}
	}

	return originals, thumbs
}

// ParseGDBGame parses a gdb.Game and returns a Game.
func ParseGDBGame(game gdb.ParsedGame) *Game {
	ret := NewGame()

	ret.ID = strconv.Itoa(game.ID)
	ret.GameTitle = game.Name
	ret.Overview = game.Overview
	ret.ReleaseDate = toXMLDate(game.ReleaseDate)
	ret.Players = int64(game.Players)

	if len(game.Developers) > 0 {
		ret.Developer = game.Developers[0].Name
	}
	if len(game.Publishers) > 0 {
		ret.Publisher = game.Publishers[0].Name
	}
	if len(game.Genres) > 0 {
		ret.Genre = game.Genres[0].Name
	}

	parsedImages, parsedThumbs := parseImages(game)
	ret.Images = parsedImages
	ret.Thumbs = parsedThumbs

	ret.Source = "theGamesDB.net"
	return ret
}

// GetName implements DS
func (g *GDB) GetName(p string) string {
	h, err := g.Hasher.Hash(p)
	if err != nil {
		return ""
	}
	name, ok := g.HM.Name(h)
	if !ok {
		return ""
	}
	return name
}

// GetGame implements DS
func (g *GDB) GetGame(ctx context.Context, p string) (*Game, error) {
	id, err := g.getID(p)
	if err != nil {
		return nil, err
	}
	resp, err := gdb.GetGame(ctx, g.APIKey, id)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}

	result := ParseGDBGame(*resp)
	return result, nil
}

func (source GDB) GetNames(ps []string) []string {
	results := make([]string, 0, len(ps))

	for _, p := range ps {
		results = append(results, source.GetName(p))
	}

	return results
}

func (source GDB) GetGames(ctx context.Context, ps []string) []GameResult {
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
