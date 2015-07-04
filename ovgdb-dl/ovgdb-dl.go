package main

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sselph/scraper/ds"
	"github.com/syndtr/goleveldb/leveldb"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	query    = "SELECT releases.releaseID, roms.romID, releases.releaseTitleName, releases.releaseCoverFront, releases.releaseDescription, releases.releaseDeveloper, releases.releasePublisher, releases.releaseGenre, releases.releaseDate, roms.romDumpSource, roms.romHashSHA1, roms.romExtensionlessFileName, releases.releaseReferenceURL FROM releases INNER JOIN roms ON roms.romID = releases.romID"
	col      = 13
	apiURL   = "https://api.github.com/repos/OpenVGDB/OpenVGDB/releases?page=1&per_page=1"
	fileName = "openvgdb.sqlite"
	zipName  = "openvgdb.zip"
	metaName = "openvgdb-s.meta"
	dbName   = "ldb"
)

func parseDate(d string) string {
	dateFormats := []string{"Jan 2, 2006", "2006", "January 2006"}
	for _, f := range dateFormats {
		t, err := time.Parse(f, d)
		if err == nil {
			return t.Format("20060102T000000")
		}
	}
	return ""
}

func getGenre(g string) string {
	return strings.SplitN(g, ",", 2)[0]
}

// scanRow scans a row. NULL values will be empty strings.
func scanRow(rows *sql.Rows) ([]string, error) {
	var s []interface{}
	for i := 0; i < col; i++ {
		s = append(s, interface{}(&sql.NullString{}))
	}
	err := rows.Scan(s...)
	if err != nil {
		return nil, err
	}
	var r []string
	for _, v := range s {
		r = append(r, v.(*sql.NullString).String)
	}
	return r, nil
}

type game struct {
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

func queryDB(db *sql.DB, q string) ([]game, error) {
	rows, err := db.Query(q)
	ret := []game{}
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		v, err := scanRow(rows)
		if err != nil {
			return ret, err
		}
		source := []string{"OpenVGDB"}
		if v[9] != "" {
			source = append(source, v[9])
		}
		if v[12] != "" {
			u, err := url.Parse(v[12])
			if err == nil {
				source = append(source, u.Host)
			}
		}
		g := game{
			ReleaseID: v[0],
			RomID:     v[1],
			Name:      v[2],
			Art:       v[3],
			Desc:      v[4],
			Developer: v[5],
			Publisher: v[6],
			Genre:     getGenre(v[7]),
			Date:      parseDate(v[8]),
			Source:    strings.Join(source, ","),
			Hash:      strings.ToLower(v[10]),
			FileName:  v[11],
		}
		// filter out rows that don't add much value.
		if g.Desc == "" && g.Art == "" && g.Developer == "" && g.Date == "" {
			continue
		}
		ret = append(ret, g)
	}
	return ret, nil
}

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

func getRelease() (*Release, error) {
	var releases []Release
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("could not get releases, %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got %v response", resp.Status)
	}

	if err = json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("could not unmarshall JSON, %v", err)
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	return &releases[0], nil
}

func updateDB(version, p string) error {
	log.Print("INFO: Checking for new OpenVGDB.")
	r, err := getRelease()
	if err != nil {
		return err
	}
	if r.TagName == version {
		log.Printf("INFO: OpenVGDB %s up to date.", version)
		return nil
	}
	log.Printf("INFO: Upgrading OpenGDB: %s -> %s.", version, r.TagName)
	if len(r.Assets) == 0 {
		return fmt.Errorf("no openvgdb found")
	}
	for _, v := range r.Assets {
		if v.Name != zipName {
			continue
		}
		resp, err := http.Get(v.DownloadURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
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
			if v.FileHeader.Name != fileName {
				continue
			}
			frc, err := v.Open()
			if err != nil {
				return err
			}
			defer frc.Close()
			b, err = ioutil.ReadAll(frc)
			if err != nil {
				return err
			}
			ioutil.WriteFile(path.Join(p, metaName), []byte(r.TagName), 0664)
			os.Remove(zf)
			log.Print("INFO: Upgrade Complete.")
			return ioutil.WriteFile(path.Join(p, fileName), b, 0664)
		}
	}
	return fmt.Errorf("no openvgdb found")
}

func mkDir(d string) error {
	fi, err := os.Stat(d)
	switch {
	case os.IsNotExist(err):
		return os.MkdirAll(d, 0775)
	case err != nil:
		return err
	case fi.IsDir():
		return nil
	}
	return fmt.Errorf("%s is a file not a directory.", d)
}

func exists(f string) bool {
	fi, err := os.Stat(f)
	return !os.IsNotExist(err) && fi.Size() > 0
}

func GetDB() (*sql.DB, error) {
	p, err := ds.DefaultCachePath()
	if err != nil {
		return nil, err
	}
	err = mkDir(p)
	var version string
	if err != nil {
		return nil, err
	}
	fp := path.Join(p, fileName)
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
	return sql.Open("sqlite3", fp)
}

func marshalGame(g game) ([]byte, error) {
	s := []string{
		g.RomID,
		g.Name,
		g.Desc,
		g.Developer,
		g.Publisher,
		g.Genre,
		g.Date,
		g.Source,
		g.Art,
	}
	return json.Marshal(s)
}

func main() {
	db, err := GetDB()
	if err != nil {
		log.Fatal(err)
	}
	games, err := queryDB(db, query)
	if err != nil {
		log.Fatal(err)
	}
	p, err := ds.DefaultCachePath()
	if err != nil {
		log.Fatal(err)
	}
	os.RemoveAll(path.Join(p, dbName))
	if err != nil {
		log.Fatal(err)
	}
	ldb, err := leveldb.OpenFile(path.Join(p, dbName), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ldb.Close()
	for _, g := range games {
		b, err := marshalGame(g)
		if err != nil {
			log.Fatal(err)
		}
		batch := new(leveldb.Batch)
		i := []byte(g.RomID)
		h := []byte(g.Hash)
		hn := []byte(g.Hash + "-name")
		fn := []byte(strings.ToLower(g.FileName))
		batch.Put(i, b)
		batch.Put(h, i)
		batch.Put(fn, i)
		batch.Put(hn, fn)
		err = ldb.Write(batch, nil)
		if err != nil {
			log.Fatal(err)
		}
	}
}
