package adb

import(
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"encoding/json"
	"io/ioutil"
)

const (
	baseURL = "http://adb.arcadeitalia.net"
	path = "/service_scraper.php"
)

var Source = "adb.arcadeitalia.net"

// ErrNotFound is returned when a game is not found.
var ErrNotFound = errors.New("rom not found")

type Result struct {
	ID string `json:"game_name"`
	Genre string `json:"genre"`
	History string `json:"history"`
	CopyRightLong string `json:"history_copyright_long"`
	CopyRightShort string `json:"history_copyright_short"`
	Manufacturer string `json:"manufacturer"`
	Players string `json:"players"`
	Name string `json:"title"`
	Cabinet string `json:"url_image_cabinet"`
	Snap string `json:"url_image_ingame"`
	Marquee string `json:"url_image_marquee"`
	Title string `json:"url_image_title"`
	Year string `json:"year"`
}

type GameResp struct {
	Results []Result `json:"result"`
}

// GetGame gets a game from mamedb.
func GetGame(name string) (*GameResp, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path
	q := u.Query()
	q.Set("ajax", "query_mame")
	q.Set("lang", "en")
	q.Set("game_name", name)
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ArcadeItalia ERR: %d: %s", resp.StatusCode, string(b))
	}
	gr := &GameResp{}
	if err := json.Unmarshal(b, gr); err != nil {
		return nil, err
	}
	return gr, nil
}
