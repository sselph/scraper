package ds

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sselph/scraper/mamedb"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	mameDBName   = "mame_history"
	mameZipURL   = "https://storage.googleapis.com/stevenselph.appspot.com/mamehist.zip"
	mameZipName  = "mamehist.zip"
	mameMetaName = "mamehist.meta"
)

// MAME is a Data Source using mamedb and arcade-history.
type MAME struct {
	db *leveldb.DB
}

func (m *MAME) GetID(p string) (string, error) {
	b := filepath.Base(p)
	id := b[:len(b)-len(filepath.Ext(b))]
	return id, nil
}

func (m *MAME) GetName(p string) string {
	return ""
}

func (m *MAME) Close() error {
	return m.db.Close()
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
	hi, err := m.db.Get([]byte(id), nil)
	if err == nil {
		desc, err := m.db.Get(hi, nil)
		if err == nil {
			game.Overview = string(desc)
		}
	}
	return game, nil
}

func updateMAMEDB(version, p string) error {
	log.Print("INFO: Checking for new MAME History.")
	req, err := http.NewRequest("GET", mameZipURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("if-none-match", version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		log.Printf("INFO: MAME History %s up to date.", version)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got %v response", resp.Status)
	}
	dbp := filepath.Join(p, mameDBName)
	err = os.RemoveAll(dbp)
	if err != nil {
		return err
	}
	err = os.Mkdir(dbp, 0775)
	if err != nil {
		return err
	}
	newVersion := resp.Header.Get("etag")
	log.Printf("INFO: Upgrading MAME History: %s -> %s.", version, newVersion)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	zf := filepath.Join(p, mameZipName)
	err = ioutil.WriteFile(zf, b, 0664)
	if err != nil {
		return err
	}
	rc, err := zip.OpenReader(zf)
	if err != nil {
		return err
	}
	defer rc.Close()
	for _, v := range rc.Reader.File {
		n := v.FileHeader.Name
		frc, err := v.Open()
		if err != nil {
			return err
		}
		defer frc.Close()
		b, err = ioutil.ReadAll(frc)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(dbp, n), b, 0664)
		if err != nil {
			return err
		}
	}
	log.Print("INFO: Upgrade Complete.")
	os.Remove(zf)
	ioutil.WriteFile(filepath.Join(p, mameMetaName), []byte(newVersion), 0664)
	return nil
}

func getMAMEDB(p string) (*leveldb.DB, error) {
	var err error
	if p == "" {
		p, err = DefaultCachePath()
		if err != nil {
			return nil, err
		}
	}
	err = mkDir(p)
	var version string
	if err != nil {
		return nil, err
	}
	fp := filepath.Join(p, mameDBName)
	mp := filepath.Join(p, mameMetaName)
	if exists(fp) && exists(mp) {
		b, err := ioutil.ReadFile(mp)
		if err != nil {
			return nil, err
		}
		version = strings.Trim(string(b[:]), "\n\r")
	}
	err = updateMAMEDB(version, p)
	if err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(fp, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// NewMAME returns a new MAME. MAME should be closed when not needed.
func NewMAME(p string) (*MAME, error) {
	db, err := getMAMEDB(p)
	if err != nil {
		return nil, err
	}
	return &MAME{db}, nil
}
