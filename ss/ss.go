package ss

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	baseURL      = "http://www.screenscraper.fr/"
	gameInfoPath = "api/jeuInfos.php"
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

// GameInfoReq is the information we use in the GameInfo command.
type GameInfoReq struct {
	Name    string
	SHA1    string
	RomType string
}

// GameNames is the name in many languages.
type GameNames struct {
	Original string `xml:"nom_ss"`
	EN       string `xml:"nom_us"`
	FR       string `xml:"nom_fr"`
	DE       string `xml:"nom_de"`
	ES       string `xml:"nom_es"`
	PT       string `xml:"nom_pt"`
}

// GameDesc is the desc in many languages.
type GameDesc struct {
	EN string `xml:"synopsis_en"`
	FR string `xml:"synopsis_fr"`
	DE string `xml:"synopsis_de"`
	ES string `xml:"synopsis_es"`
	PT string `xml:"synopsis_pt"`
}

// GameGenre is the genre in many languages.
type GameGenre struct {
	EN []string `xml:"genres_en>genre_en"`
	FR []string `xml:"genres_fr>genre_fr"`
	DE []string `xml:"genres_de>genre_de"`
	ES []string `xml:"genres_es>genre_es"`
	PT []string `xml:"genres_pt>genre_pt"`
}

// GameMedia is the media for many regions.
type GameMedia struct {
	ScreenShot    string `xml:"media_screenshot"`
	ScreenMarquee string `xml:"media_screenmarquee"`
	Marquee       string `xml:"media_marquee"`
	BoxUS         string `xml:"media_boxs>media_boxs2d>media_box2d_us"`
	BoxWOR        string `xml:"media_boxs>media_boxs2d>media_box2d_wor"`
	BoxFR         string `xml:"media_boxs>media_boxs2d>media_box2d_fr"`
	BoxEU         string `xml:"media_boxs>media_boxs2d>media_box2d_eu"`
	BoxJP         string `xml:"media_boxs>media_boxs2d>media_box2d_jp"`
	BoxXX         string `xml:"media_boxs>media_boxs2d>media_box2d_xx"`
	Box3DUS       string `xml:"media_boxs>media_boxs3d>media_box3d_us"`
	Box3DWOR      string `xml:"media_boxs>media_boxs3d>media_box3d_wor"`
	Box3DFR       string `xml:"media_boxs>media_boxs3d>media_box3d_fr"`
	Box3DEU       string `xml:"media_boxs>media_boxs3d>media_box3d_eu"`
	Box3DJP       string `xml:"media_boxs>media_boxs3d>media_box3d_jp"`
	Box3DXX       string `xml:"media_boxs>media_boxs3d>media_box3d_xx"`
	FlyerUS       string `xml:"media_flyers>media_flyer_us"`
	FlyerWOR      string `xml:"media_flyers>media_flyer_wor"`
	FlyerFR       string `xml:"media_flyers>media_flyer_fr"`
	FlyerEU       string `xml:"media_flyers>media_flyer_eu"`
	FlyerJP       string `xml:"media_flyers>media_flyer_jp"`
	FlyerXX       string `xml:"media_flyers>media_flyer_xx"`
}

// GameDates is the date for many regions.
type GameDates struct {
	US  string `xml:"date_us"`
	WOR string `xml:"date_wor"`
	FR  string `xml:"date_fr"`
	EU  string `xml:"date_eu"`
	JP  string `xml:"date_jp"`
	XX  string `xml:"date_xx"`
}

// Game represents a game in SS.
type Game struct {
	ID        int       `xml:"id"`
	Name      string    `xml:"nom"`
	Names     GameNames `xml:"noms"`
	Region    string    `xml:"region"`
	Publisher string    `xml:"editeur"`
	Developer string    `xml:"developpeur"`
	Players   string    `xml:"joueurs"`
	Rating    string    `xml:"note"`
	Desc      GameDesc  `xml:"synopsis"`
	Genre     GameGenre `xml:"genres"`
	Media     GameMedia `xml:"medias"`
	Dates     GameDates `xml:"dates"`
}

// GameInfoResp is the response from GameInfo.
type GameInfoResp struct {
	Game Game `xml:"jeu"`
}

// GameInfo is the call to get game info.
func GameInfo(dev DevInfo, user UserInfo, req GameInfoReq) (*GameInfoResp, error) {
	u, err := url.Parse(baseURL)
	u.Path = gameInfoPath
	q := url.Values{}
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
		v := u.Query()
		v.Set("devid", "xxx")
		v.Set("devpassword", "yyy")
		v.Set("softname", "zzz")
		u.RawQuery = v.Encode()
		return nil, fmt.Errorf("getting game url:%s, error:%s", u, err)
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
	if err := xml.Unmarshal(b, r); err != nil {
		return nil, err
	}
	return r, nil
}
