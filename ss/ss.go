package ss

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// JSON field prefixes.
const (
	pre2D       = "media_box2d_"
	pre3D       = "media_box3d_"
	preFlyer    = "media_flyer_"
	preDate     = "date_"
	preGenre    = "genres_"
	preSynopsis = "synopsis_"
)

const (
	baseURL      = "http://www.screenscraper.fr/"
	gameInfoPath = "api/jeuInfos.php"
	userInfoPath = "api/ssuserInfos.php"
)

// ErrNotFound is the error returned when a ROM isn't found.
var ErrNotFound = errors.New("not found")

// DevInfo is the information about the developer and used across APIs.
type DevInfo struct {
	ID       string
	Password string
	Name     string
}

// UserInfo is information about the user making the call.
type UserInfo struct {
	ID       string
	Password string
}

type UserInfoResp struct {
	ID              string `xml:"ssuser>id"`
	Level           int    `xml:"ssuser>niveau"`
	Contribution    int    `xml:"ssuser>contribution"`
	UploadedSystems int    `xml:"ssuser>uploadsysteme"`
	UploadedInfo    int    `xml:"ssuser>uploadinfos"`
	ROMsAssociated  int    `xml:"ssuser>romasso"`
	UpdatedMedia    int    `xml:"ssuser>uploadmedia"`
	FavoriteRegion  string `xml:"ssuser>favregion"`
	MaxThreads      int    `xml:"ssuser>maxthreads"`
}

// GameInfoReq is the information we use in the GameInfo command.
type GameInfoReq struct {
	Name    string
	SHA1    string
	RomType string
}

type SafeStringMap struct {
	Map map[string]string
}

func (s *SafeStringMap) UnmarshalJSON(b []byte) error {
	if s.Map == nil {
		s.Map = make(map[string]string)
	}
	x := make(map[string]json.RawMessage)
	if err := json.Unmarshal(b, &x); err != nil {
		log.Print("json: %v", err)
		return nil
	}
	for k, v := range x {
		var y string
		if err := json.Unmarshal(v, &y); err == nil {
			s.Map[k] = y
		}
	}
	return nil
}

type BoxArt struct {
	Box2D SafeStringMap `json:"media_boxs2d"`
	Box3D SafeStringMap `json:"media_boxs3d"`
}

type Media struct {
	Screenshot    string        `json:"media_screenshot"`
	ScreenMarquee string        `json:"media_screenmarquee"`
	Marquee       string        `json:"media_marquee"`
	Video         string        `json:"media_video"`
	Flyers        SafeStringMap `json:"media_flyers"`
	BoxArt        BoxArt        `json:"media_boxs"`
}

func getPrefix(m map[string]string, pre string) (string, bool) {
	for k, v := range m {
		if strings.HasPrefix(k, pre) && !strings.Contains(strings.TrimPrefix(k, pre), "_") {
			return v, true
		}
	}
	return "", false
}

func getSuffix(m map[string]string, pre string, suf []string) (string, bool) {
	if m == nil {
		return "", false
	}
	for _, x := range suf {
		if i, ok := m[pre+x]; ok {
			return i, true
		}
		if x == "xx" {
			if i, ok := getPrefix(m, pre); ok {
				return i, true
			}
		}
	}
	return "", false
}

func (m Media) Box2D(r []string) (string, bool) {
	return getSuffix(m.BoxArt.Box2D.Map, pre2D, r)
}

func (m Media) Box3D(r []string) (string, bool) {
	return getSuffix(m.BoxArt.Box3D.Map, pre3D, r)
}

func (m Media) Flyer(r []string) (string, bool) {
	return getSuffix(m.Flyers.Map, preFlyer, r)
}

type ROM struct {
	FileName   string `json:"romfilename"`
	SHA1       string `json:"romsha1"`
	RegionsRaw string `json:"romregions"`
}

func (r ROM) Regions() []string {
	var x []string
	for _, p := range strings.Split(r.RegionsRaw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		x = append(x, p)
	}
	return x
}

type Game struct {
	Synopsis  SafeStringMap              `json:"synopsis"`
	ID        string                     `json:"id"`
	Name      string                     `json:"nom"`
	Names     SafeStringMap              `json:"noms"`
	Regions   []string                   `json:"regionshortnames"`
	Publisher string                     `json:"editeur"`
	Developer string                     `json:"developpeur"`
	Players   string                     `json:"joueurs"`
	Rating    string                     `json:"note"`
	Dates     SafeStringMap              `json:"dates"`
	Genres    map[string]json.RawMessage `json:"genres:`
	Media     Media                      `json:"medias"`
	ROMs      []ROM                      `json:"roms"`
	genres    map[string]string
}

func (g Game) Date(r []string) (string, bool) {
	return getSuffix(g.Dates.Map, preDate, r)
}

func (g *Game) decodeGenre() {
	g.genres = make(map[string]string)
	for k, v := range g.Genres {
		if strings.HasSuffix(k, "_medias") || strings.HasSuffix(k, "_id") {
			continue
		}
		s := []string{}
		if err := json.Unmarshal(v, &s); err == nil {
			g.genres[k] = strings.Join(s, " / ")
		}
	}
}

func (g Game) Genre(l []string) (string, bool) {
	if g.genres == nil {
		g.decodeGenre()
	}
	return getSuffix(g.genres, preGenre, l)
}

func (g Game) Desc(l []string) (string, bool) {
	return getSuffix(g.Synopsis.Map, preSynopsis, l)
}

func (g Game) ROM(req GameInfoReq) ROM {
	for _, x := range g.ROMs {
		if strings.ToLower(x.SHA1) == strings.ToLower(req.SHA1) {
			return x
		}
	}
	for _, x := range g.ROMs {
		if x.FileName == req.Name {
			return x
		}
	}
	return ROM{}
}

type Response struct {
	Game Game `json:"jeu"`
}

type GameInfoResp struct {
	Response Response `json:"response"`
}

func SanitizeURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	v := u.Query()
	v.Set("devid", "xxx")
	v.Set("devpassword", "yyy")
	v.Set("softname", "zzz")
	v.Del("ssid")
	v.Del("sspassword")
	u.RawQuery = v.Encode()
	return u.String()
}

func User(dev DevInfo, user UserInfo) (*UserInfoResp, error) {
	u, err := url.Parse(baseURL)
	u.Path = userInfoPath
	q := url.Values{}
	q.Set("output", "xml")
	q.Set("devid", dev.ID)
	q.Set("devpassword", dev.Password)
	if dev.Name != "" {
		q.Set("softname", dev.Name)
	}
	q.Set("ssid", user.ID)
	q.Set("sspassword", user.Password)
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		v := u.Query()
		v.Set("devid", "xxx")
		v.Set("devpassword", "yyy")
		v.Set("softname", "zzz")
		u.RawQuery = v.Encode()
		return nil, fmt.Errorf("getting game url:%s, error:%s", u, err)
	}
	defer resp.Body.Close()
	r := &UserInfoResp{}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := xml.Unmarshal(b, r); err != nil {
		return nil, fmt.Errorf("ss: cannot parse response: %q", err)
	}
	return r, nil
}

func Threads(dev DevInfo, user UserInfo) int {
	if user.ID == "" || user.Password == "" {
		return 1
	}
	i, err := User(dev, user)
	if err != nil {
		log.Print("error getting allowed threads defaulting to 1")
		return 1
	}
	if i.MaxThreads < 1 {
		return 1
	}
	return i.MaxThreads
}

// GameInfo is the call to get game info.
func GameInfo(dev DevInfo, user UserInfo, req GameInfoReq) (*GameInfoResp, error) {
	u, err := url.Parse(baseURL)
	u.Path = gameInfoPath
	q := url.Values{}
	q.Set("output", "json")
	q.Set("devid", dev.ID)
	q.Set("devpassword", dev.Password)
	if dev.Name != "" {
		q.Set("softname", dev.Name)
	}
	if user.ID != "" {
		q.Set("ssid", user.ID)
	}
	if user.Password != "" {
		q.Set("sspassword", user.Password)
	}
	if req.SHA1 != "" {
		q.Set("sha1", req.SHA1)
	}
	if req.RomType == "" {
		q.Set("romtype", "rom")
	} else {
		q.Set("romtype", req.RomType)
	}
	if req.Name != "" {
		q.Set("romnom", req.Name)
	}
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		if uerr, ok := err.(*url.Error); ok {
			uerr.URL = SanitizeURL(uerr.URL)
			return nil, uerr
		}
		return nil, err
	}
	defer resp.Body.Close()
	r := &GameInfoResp{}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(b, []byte("Erreur : Rom/Iso/Dossier non trouv")) {
		return nil, ErrNotFound
	}
	if bytes.HasPrefix(b, []byte("Erreur : Jeu non trouv!")) {
		return nil, ErrNotFound
	}
	if err := json.Unmarshal(b, r); err != nil {
		if err.Error() == "invalid character 'm' looking for beginning of value" {
			return nil, fmt.Errorf("ss: %s", string(b))
		}
		if err.Error() == "invalid character 'A' looking for beginning of value" {
			return nil, fmt.Errorf("ss: %s", string(b))
		}
		return nil, fmt.Errorf("ss: cannot parse response: %q", err)
	}
	return r, nil
}
