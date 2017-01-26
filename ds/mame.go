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

// getID gets the ID for the game..
func (m *MAME) getID(p string) (string, error) {
	b := filepath.Base(p)
	id := b[:len(b)-len(filepath.Ext(b))]
	return id, nil
}

// GetName implements DS.
func (m *MAME) GetName(p string) string {
	return ""
}

// Close implements io.Closer.
func (m *MAME) Close() error {
	return m.db.Close()
}

// GetGame implements DS.
func (m *MAME) GetGame(p string) (*Game, error) {
	id, err := m.getID(p)
	if err != nil {
		return nil, err
	}
	g, err := mamedb.GetGame(id)
	if err != nil {
		if err == mamedb.ErrNotFound {
			return nil, ErrNotFound
		}
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
	if version != "" && resp.StatusCode == 429 {
		log.Printf("WARN: Using cached MAME History. Server over quota.")
		return nil
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

func getMAMEDB(p string, u bool) (*leveldb.DB, error) {
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
	if !exists(fp) || u {
		err = updateMAMEDB(version, p)
		if err != nil {
			return nil, err
		}
	}
	db, err := leveldb.OpenFile(fp, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// NewMAME returns a new MAME. MAME should be closed when not needed.
func NewMAME(p string, u bool) (*MAME, error) {
	db, err := getMAMEDB(p, u)
	if err != nil {
		return nil, err
	}
	return &MAME{db}, nil
}
