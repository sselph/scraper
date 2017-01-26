package ds

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	zipURL   = "https://storage.googleapis.com/stevenselph.appspot.com/openvgdb2.zip"
	dbName   = "ldb"
	zipName  = "openvgdb.zip"
	metaName = "openvgdb.meta"
)

func ovgdbUnmarshalGame(b []byte) (*Game, error) {
	var s []string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return nil, err
	}
	if len(s) != 9 {
		return nil, fmt.Errorf("length of slice must be 9 but was %d", len(s))
	}
	g := NewGame()
	g.ID = s[0]
	g.GameTitle = s[1]
	g.Overview = s[2]
	g.Developer = s[3]
	g.Publisher = s[4]
	g.Genre = s[5]
	g.ReleaseDate = s[6]
	g.Source = s[7]
	if s[8] != "" {
		g.Images[ImgBoxart] = HTTPImage{s[8]}
		g.Thumbs[ImgBoxart] = HTTPImage{s[8]}
	}
	return g, nil
}

// OVGDB is a DataSource using OpenVGDB.
type OVGDB struct {
	db     *leveldb.DB
	Hasher *Hasher
}

// GetName implements DS.
func (o *OVGDB) GetName(p string) string {
	h, err := o.Hasher.Hash(p)
	if err != nil {
		return ""
	}
	n, err := o.db.Get([]byte(h+"-name"), nil)
	if err != nil {
		return ""
	}
	return string(n)
}

// getID gets the ID from the path.
func (o *OVGDB) getID(p string) (string, error) {
	h, err := o.Hasher.Hash(p)
	if err != nil {
		return "", err
	}
	id, err := o.db.Get([]byte(h), nil)
	if err == nil {
		return string(id), nil
	}
	if err != nil && err != leveldb.ErrNotFound {
		return "", err
	}
	b := filepath.Base(p)
	n := b[:len(b)-len(filepath.Ext(b))]
	id, err = o.db.Get([]byte(strings.ToLower(n)), nil)
	if err != nil {
		return "", ErrNotFound
	}
	return string(id), nil
}

// GetGame implements DS.
func (o *OVGDB) GetGame(p string) (*Game, error) {
	id, err := o.getID(p)
	if err != nil {
		return nil, err
	}
	g, err := o.db.Get([]byte(id), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return ovgdbUnmarshalGame(g)
}

// Close closes the DB.
func (o *OVGDB) Close() error {
	return o.db.Close()
}

func updateDB(version, p string) error {
	log.Print("INFO: Checking for new OpenVGDB.")
	req, err := http.NewRequest("GET", zipURL, nil)
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
		log.Printf("INFO: OpenVGDB %s up to date.", version)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got %v response", resp.Status)
	}
	if version != "" && resp.StatusCode == 429 {
		log.Printf("WARN: Using cached OpenVGDB. Server over quota.")
		return nil
	}
	dbp := path.Join(p, dbName)
	err = os.RemoveAll(dbp)
	if err != nil {
		return err
	}
	err = os.Mkdir(dbp, 0775)
	if err != nil {
		return err
	}
	newVersion := resp.Header.Get("etag")
	log.Printf("INFO: Upgrading OpenGDB: %s -> %s.", version, newVersion)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	zf := path.Join(p, zipName)
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
		err = ioutil.WriteFile(path.Join(dbp, n), b, 0664)
		if err != nil {
			return err
		}
	}
	log.Print("INFO: Upgrade Complete.")
	os.Remove(zf)
	ioutil.WriteFile(path.Join(p, metaName), []byte(newVersion), 0664)
	return nil
}

func getDB(p string, u bool) (*leveldb.DB, error) {
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
	fp := path.Join(p, dbName)
	mp := path.Join(p, metaName)
	if exists(fp) && exists(mp) {
		b, err := ioutil.ReadFile(mp)
		if err != nil {
			return nil, err
		}
		version = strings.Trim(string(b[:]), "\n\r")
	}
	if !exists(fp) || u {
		err = updateDB(version, p)
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

// NewOVGDB returns a new OVGDB. OVGDB should be closed when not needed.
func NewOVGDB(h *Hasher, u bool) (*OVGDB, error) {
	db, err := getDB("", u)
	if err != nil {
		return nil, err
	}
	return &OVGDB{Hasher: h, db: db}, nil
}
