package ovgdb

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/syndtr/goleveldb/leveldb"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
)

const (
	zipURL   = "https://storage.googleapis.com/stevenselph.appspot.com/openvgdb.zip"
	dbName   = "ldb"
	zipName  = "openvgdb.zip"
	metaName = "openvgdb.meta"
)

type Game struct {
	ReleaseID string
	RomID     string
	Name      string
	Art       string
	Desc      string
	Developer string
	Publisher string
	Genre     string
	Date      string
	Source    string
	Hash      string
	FileName  string
}

func (g *Game) ToSlice() []string {
	return []string{
		g.ReleaseID,
		g.RomID,
		g.Name,
		g.Art,
		g.Desc,
		g.Developer,
		g.Publisher,
		g.Genre,
		g.Date,
		g.Source,
		g.Hash,
		g.FileName,
	}
}

func GameFromSlice(s []string) (*Game, error) {
	if len(s) != 12 {
		return nil, fmt.Errorf("length of slice must be 12 but was %s", len(s))
	}
	g := &Game{
		ReleaseID: s[0],
		RomID:     s[1],
		Name:      s[2],
		Art:       s[3],
		Desc:      s[4],
		Developer: s[5],
		Publisher: s[6],
		Genre:     s[7],
		Date:      s[8],
		Source:    s[9],
		Hash:      s[10],
		FileName:  s[11],
	}
	return g, nil
}

type DB struct {
	db *leveldb.DB
}

func (db *DB) GetGame(n string) (*Game, error) {
	v, err := db.db.Get([]byte(n), nil)
	if err != nil {
		return nil, err
	}
	g, err := db.db.Get(v, nil)
	if err != nil {
		return nil, err
	}
	var s []string
	err = json.Unmarshal(g, &s)
	if err != nil {
		return nil, err
	}
	return GameFromSlice(s)
}

func (db *DB) Close() error {
	return db.db.Close()
}

func GetDBPath() (string, error) {
	hd, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return path.Join(hd, "Application Data", "sselph-scraper"), nil
	} else {
		return path.Join(hd, ".sselph-scraper"), nil
	}
}

func mkDir(d string) error {
	fi, err := os.Stat(d)
	switch {
	case os.IsNotExist(err):
		return os.MkdirAll(d, 0777)
	case err != nil:
		return err
	case fi.IsDir():
		return nil
	}
	return fmt.Errorf("%s is a file not a directory.", d)
}

func exists(f string) bool {
	_, err := os.Stat(f)
	return !os.IsNotExist(err)
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
	dbp := path.Join(p, dbName)
	err = os.RemoveAll(dbp)
	if err != nil {
		return err
	}
	err = os.Mkdir(dbp, 0777)
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
	err = ioutil.WriteFile(zf, b, 0777)
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
		err = ioutil.WriteFile(path.Join(dbp, n), b, 0777)
		if err != nil {
			return err
		}
	}
	log.Print("INFO: Upgrade Complete.")
	os.Remove(zf)
	ioutil.WriteFile(path.Join(p, metaName), []byte(newVersion), 0777)
	return nil
}

func GetDB() (*DB, error) {
	p, err := GetDBPath()
	if err != nil {
		return nil, err
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
	err = updateDB(version, p)
	if err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(fp, nil)
	if err != nil {
		return nil, err
	}
	return &DB{db}, nil
}
