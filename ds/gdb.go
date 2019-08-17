package ds

import (
	"context"
	"fmt"
	"strconv"
	"time"

	gamesdb "github.com/J-Swift/thegamesdb-swagger-client-go"
	"github.com/sselph/scraper/gdb"
)

// GDB is a DataSource using thegamesdb.net
type GDB struct {
	HM     *HashMap
	Hasher *Hasher
	APIKey string
}

// getFront gets the front boxart for a Game if it exists.
func getFront(images []gamesdb.GameImage) *gamesdb.GameImage {
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
	req := gdb.GGReq{ID: id}
	resp, err := gdb.GetGame(ctx, g.APIKey, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Games) == 0 {
		return nil, fmt.Errorf("game with id (%s) not found", id)
	}
	game := resp.Games[0]
	ret := ParseGDBGame(game, resp.Boxart, resp.ImageBaseURL)
	ret.ID = id
	return ret, nil
}

type ImageTypeName string

func bucketImagesByType(images []gamesdb.GameImage) map[ImageTypeName][]gamesdb.GameImage {
	res := make(map[ImageTypeName][]gamesdb.GameImage)

	for _, image := range images {
		imageType := ImageTypeName(image.Type)
		existing := res[imageType]

		existing = append(existing, image)
		res[imageType] = existing
	}

	return res
}

func parseImages(game gamesdb.Game, images map[string][]gamesdb.GameImage, baseUrls gamesdb.ImageBaseUrlMeta) (map[ImgType]Image, map[ImgType]Image) {
	originals := make(map[ImgType]Image)
	thumbs := make(map[ImgType]Image)

	baseURLOriginal := baseUrls.Original
	baseURLThumb := baseUrls.Thumb

	gameImages := images[strconv.Itoa(int(game.Id))]

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
func ParseGDBGame(game gamesdb.Game, images map[string][]gamesdb.GameImage, baseUrls gamesdb.ImageBaseUrlMeta) *Game {
	ret := NewGame()

	parsedImages, parsedThumbs := parseImages(game, images, baseUrls)
	ret.Images = parsedImages
	ret.Thumbs = parsedThumbs

	//var genre string
	//if len(game.Genres) >= 1 {
	//genre = game.Genres[0]
	//}
	ret.GameTitle = game.GameTitle
	ret.Overview = game.Overview
	// ret.Rating = game.Rating / 10.0
	ret.ReleaseDate = toXMLDate(game.ReleaseDate)
	//ret.Developer = game.Developer
	//ret.Publisher = game.Publisher
	//ret.Genre = genre
	ret.Source = "theGamesDB.net"
	ret.Players = int64(game.Players)
	return ret
}
