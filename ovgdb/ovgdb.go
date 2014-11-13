package ovgdb

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/go-homedir"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

const (
	hashQuery = "SELECT releases.releaseTitleName, releases.releaseCoverFront, releases.releaseDescription, releases.releaseDeveloper, releases.releasePublisher, releases.releaseGenre, releases.releaseDate FROM releases INNER JOIN roms ON roms.romID = releases.romID WHERE LOWER(roms.romHashSHA1) = \"%s\""
	nameQuery = "SELECT releases.releaseTitleName, releases.releaseCoverFront, releases.releaseDescription, releases.releaseDeveloper, releases.releasePublisher, releases.releaseGenre, releases.releaseDate FROM releases INNER JOIN roms ON roms.romID = releases.romID WHERE roms.romExtensionlessFileName = \"%s\""
	col       = 7
	apiURL    = "https://api.github.com/repos/OpenVGDB/OpenVGDB/releases?page=1&per_page=1"
	fileName  = "openvgdb.sqlite"
	zipName   = "openvgdb.zip"
	metaName  = "openvgdb.meta"
)

type Game struct {
	Name      string
	Art       string
	Desc      string
	Developer string
	Publisher string
	Genre     string
	Date      string
}

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

func queryDB(db *sql.DB, q string) ([]Game, error) {
	rows, err := db.Query(q)
	ret := []Game{}
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		v, err := scanRow(rows)
		if err != nil {
			return ret, err
		}
		g := Game{v[0], v[1], v[2], v[3], v[4], getGenre(v[5]), parseDate(v[6])}
		ret = append(ret, g)
	}
	return ret, nil
}

func GetGamesFromName(db *sql.DB, n string) ([]Game, error) {
	return queryDB(db, fmt.Sprintf(nameQuery, n))
}

func GetGamesFromHash(db *sql.DB, h string) ([]Game, error) {
	return queryDB(db, fmt.Sprintf(hashQuery, strings.ToLower(h)))
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
		err = ioutil.WriteFile(zf, b, 0777)
		if err != nil {
			return err
		}
		rc, err := zip.OpenReader(zf)
		if err != nil {
			return err
		}
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
			ioutil.WriteFile(path.Join(p, metaName), []byte(r.TagName), 0777)
			os.Remove(zf)
			log.Print("INFO: Upgrade Complete.")
			return ioutil.WriteFile(path.Join(p, fileName), b, 0777)
		}
	}
	return fmt.Errorf("no openvgdb found")
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

func validTemp(s string) bool {
	fi, err := os.Stat(s)
	if err != nil || fi.Size() == 0 {
		return false
	}
	n := time.Now()
	t := fi.ModTime().Add(60 * time.Minute)
	if t.Before(n) {
		return false
	}
	return true
}

func exists(f string) bool {
	fi, err := os.Stat(f)
	return !os.IsNotExist(err) && fi.Size() > 0
}

func GetDB() (*sql.DB, error) {
	p, err := GetDBPath()
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
