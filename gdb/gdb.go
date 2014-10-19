/*
Copyright (c) 2014 sselph

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package gdb interacts with thegamedb.net's API.
// 
// Example:
//  resp, err := gdb.GetGame(GGReq{ID: "5"})
package gdb

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
)

const (
	GDBURL = "http://thegamesdb.net"
	GGPath = "/api/GetGame.php"
	GGLPath = "/api/GetGamesList.php"
)

type GGLReq struct {
	Name     string
	Platform string
	Genre    string
}

type GGLResp struct {
	XMLName xml.Name
	Game    []GameTrunc
	err     string `xml:",chardata"`
}

// GameTrunc is used to parse the GetGamesList's <Game> tag.
type GameTrunc struct {
	ID          string `xml:"id"`
	GameTitle   string
	ReleaseDate string
	Platform    string
}

//GGReq is the request to GetGame.
type GGReq struct {
	ID       string
	Name     string
	Platform string
}

// GGResp is the response of the GetGame query.
type GGResp struct {
	XMLName  xml.Name
	ImageURL string `xml:"baseImgUrl"`
	Game     []Game
	err      string `xml:",chardata"`
}

// Image is used to parse the GetGame's <Images> tags.
type Image struct {
	URL    string `xml:",chardata"`
	Width  uint   `xml:"width,attr"`
	Height uint   `xml:"height,attr"`
	// Only used on boxart.
	Side  string `xml:"side,attr"`
	Thumb string `xml:"thumb,attr"`
}

// OImage is used to parse <Images> tags for fanart and
// screenshots since they are formatted differently.
type OImage struct {
	Original Image  `xml:"original"`
	Thumb    string `xml:"thumb"`
}

// Game is used to parse the GetGame's <Game> tag.
type Game struct {
	ID          string `xml:"id"`
	GameTitle   string
	Overview    string
	ReleaseDate string
	Platform    string
	Developer   string
	Publisher   string
	Genres      []string `xml:"Genres>genre"`
	Players     string
	Rating      float64
	ESRB        string
	AltTitles   []string `xml:"AlternateTitles>title"`
	BoxArt      []Image  `xml:"Images>boxart"`
	ClearLogo   []Image  `xml:"Images>clearlogo"`
	Banner      []Image  `xml:"Images>banner"`
	FanArt      []OImage `xml:"Images>fanart"`
	Screenshot  []OImage `xml:"Images>screenshot"`
}

// GetGame gets the game information from the DB.
func GetGame(req GGReq) (*GGResp, error) {
	u, err := url.Parse(GDBURL)
	u.Path = GGPath
	q := url.Values{}
	switch {
	case req.ID != "":
		q.Set("id", req.ID)
	case req.Name != "":
		q.Set("name", req.Name)
		if req.Platform != "" {
			q.Set("platform", req.Platform)
		}
	default:
		return nil, fmt.Errorf("must provide an ID or Name.")
	}
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("getting game url:%s, error:%s", u, err)
	}
	defer resp.Body.Close()
	r := &GGResp{}
	decoder := xml.NewDecoder(resp.Body)
	if err := decoder.Decode(r); err != nil {
		return nil, err
	}
	if r.XMLName.Local == "Error" {
		return nil, fmt.Errorf("GetGame error: %s", r.err)
	} else {
		r.err = ""
	}
	return r, nil
}

// GetGameList gets the game information from the DB.
func GetGameList(req GGLReq) (*GGLResp, error) {
	u, err := url.Parse(GDBURL)
	u.Path = GGLPath
	q := url.Values{}
	if req.Name == "" {
		return nil, fmt.Errorf("must provide Name")
	}
	q.Set("name", req.Name)
	if req.Platform != "" {
		q.Set("platform", req.Platform)
	}
	if req.Genre != "" {
		q.Set("genre", req.Genre)
	}
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("getting game list url:%s, error:%s", u, err)
	}
	defer resp.Body.Close()
	r := &GGLResp{}
	decoder := xml.NewDecoder(resp.Body)
	if err := decoder.Decode(r); err != nil {
		return nil, err
	}
	if r.XMLName.Local == "Error" {
		return nil, fmt.Errorf("GetGameList error: %s", r.err)
	} else {
		r.err = ""
	}
	return r, nil
}

// IsUp returns if thegamedb.net is up.
func IsUp() bool {
	u, err := url.Parse(GDBURL)
	u.Path = GGPath
	q := url.Values{}
	q.Set("id", "1")
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return false
	}
	if resp.StatusCode != 200 {
		return false
	}
	defer resp.Body.Close()
	r := &GGResp{}
	decoder := xml.NewDecoder(resp.Body)
	if err := decoder.Decode(r); err != nil {
		return false
	}
	return true
}
