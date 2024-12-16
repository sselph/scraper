package ss

import (
	"bytes"
	"context"
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

type MediaType string

const (
	Screenshot    MediaType = "ss"
	ScreenMarquee MediaType = "screenmarquee"
	Marquee       MediaType = "marquee"
	Video         MediaType = "video"
	Box2D         MediaType = "box-2D"
	Box3D         MediaType = "box-3D"
	Flyer         MediaType = "flyer"
	Wheel         MediaType = "wheel"
	Support2D     MediaType = "support-2D"
	SupportLabel  MediaType = "support-texture"
)

const (
	baseURL      = "https://www.screenscraper.fr/"
	gameInfoPath = "api2/jeuInfos.php"
	userInfoPath = "api2/ssuserInfos.php"
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

type Medium struct {
	Type   MediaType `json:"type"`
	Parent string    `json:"parent"`
	URL    string    `json:"url"`
	Format string    `json:"format"`
	Region string    `json:"region"`
}

func (game Game) MediaWithFormat(mediaType MediaType, regions []string) (string, string, bool) {
	if game.Medias == nil {
		return "", "", false
	}
	for _, region := range regions {
		for _, medium := range game.Medias {
			if medium.Parent == "jeu" && medium.Type == mediaType && (medium.Region == region || medium.Region == "" && region == "xx") {
				return medium.URL, medium.Format, true
			}
		}
	}
	return "", "", false
}

func (game Game) Media(mediaType MediaType, regions []string) (string, bool) {
	url, _, ok := game.MediaWithFormat(mediaType, regions)
	return url, ok
}

type ROM struct {
	FileName   string `json:"romfilename"`
	SHA1       string `json:"romsha1"`
	RegionsRaw string `json:"romregions"`
}

type LanguageAndText struct {
	Language string `json:"langue"`
	Text     string `json:"text"`
}

type IDAndText struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type RegionAndText struct {
	Region string `json:"region"`
	Text   string `json:"text"`
}

type TextField struct {
	Text string `json:"text"`
}

type Genre struct {
	ID    string            `json:"id"`
	Names []LanguageAndText `json:"noms"`
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
	ID           string            `json:"id"`
	Names        []RegionAndText   `json:"noms"`
	Descriptions []LanguageAndText `json:"synopsis"`
	Publisher    IDAndText         `json:"editeur"`
	Developer    IDAndText         `json:"developpeur"`
	Players      TextField         `json:"joueurs"`
	Rating       TextField         `json:"note"`
	Dates        []RegionAndText   `json:"dates"`
	Genres       []Genre           `json:"genres"`
	Medias       []Medium          `json:"medias"`
	ROMs         []ROM             `json:"roms"`
	names        map[string]string
	descriptions map[string]string
	genres       map[string]string
	dates        map[string]string
}

func (g *Game) decodeGenre() {
	g.genres = make(map[string]string)
	for _, genre := range g.Genres {
		for _, name := range genre.Names {
			if g.genres[name.Language] != "" {
				g.genres[name.Language] = g.genres[name.Language] + " / " + name.Text
			} else {
				g.genres[name.Language] = name.Text
			}
		}
	}
}

func (g *Game) decodeNames() {
	g.names = make(map[string]string)
	for _, name := range g.Names {
		g.names[name.Region] = name.Text
	}
}

func (g *Game) decodeDescriptions() {
	g.descriptions = make(map[string]string)
	for _, description := range g.Descriptions {
		g.descriptions[description.Language] = description.Text
	}
}

func (g *Game) decodeDates() {
	g.dates = make(map[string]string)
	for _, date := range g.Dates {
		g.dates[date.Region] = date.Text
	}
}

func getFirstMatch(mapping map[string]string, keys []string) (string, bool) {
	for _, key := range keys {
		if mapping[key] != "" {
			return mapping[key], true
		}
	}
	return "", false
}

func (g Game) Genre(l []string) (string, bool) {
	if g.genres == nil {
		g.decodeGenre()
	}
	return getFirstMatch(g.genres, l)
}

func (g Game) Name(r []string) (string, bool) {
	if g.names == nil {
		g.decodeNames()
	}
	return getFirstMatch(g.names, r)
}

func (g Game) Desc(l []string) (string, bool) {
	if g.descriptions == nil {
		g.decodeDescriptions()
	}
	return getFirstMatch(g.descriptions, l)
}

func (g Game) Date(r []string) (string, bool) {
	if g.dates == nil {
		g.decodeDates()
	}
	return getFirstMatch(g.dates, r)
}

func (g Game) ROM(req GameInfoReq) (ROM, bool) {
	for _, x := range g.ROMs {
		if strings.ToLower(x.SHA1) == strings.ToLower(req.SHA1) {
			return x, true
		}
	}
	for _, x := range g.ROMs {
		if x.FileName == req.Name {
			return x, true
		}
	}
	return ROM{}, false
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

func User(ctx context.Context, dev DevInfo, user UserInfo) (*UserInfoResp, error) {
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
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
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

func Threads(ctx context.Context, dev DevInfo, user UserInfo) int {
	if user.ID == "" || user.Password == "" {
		return 1
	}
	i, err := User(ctx, dev, user)
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
func GameInfo(ctx context.Context, dev DevInfo, user UserInfo, req GameInfoReq) (*GameInfoResp, error) {
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
	q.Set("romnom", "0")
	if req.Name != "" {
		q.Set("romnom", req.Name)
	}
	u.RawQuery = q.Encode()
	hReq, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	hReq = hReq.WithContext(ctx)
	resp, err := http.DefaultClient.Do(hReq)
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
	if bytes.HasPrefix(b, []byte("Erreur : Jeu non trouv")) {
		return nil, ErrNotFound
	}
	if err := json.Unmarshal(b, r); err != nil {
		if strings.HasPrefix(err.Error(), "invalid character '") && strings.HasSuffix(err.Error(), "' looking for beginning of value") {
			return nil, fmt.Errorf("ss: %s", string(b))
		}
		return nil, fmt.Errorf("ss: cannot parse response: %q", err)
	}
	return r, nil
}
