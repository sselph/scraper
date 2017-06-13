/*Copyright (c) 2014 Alec Lofquist

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.*/

// This code was adapted from https://github.com/Aloshi/EmulationStation/pull/200

package mamedb

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
)

const (
	baseURL = "http://mamedb.blu-ferret.co.uk"
	path    = baseURL + "/game/"
)

var (
	infoLineRE   = regexp.MustCompile("<h1>Game Details</h1>(.*?)Clock Speed")
	titleRE      = regexp.MustCompile("<b>Name:&nbsp</b>(?P<title>.*?)<br/>.*?<b>Year:&nbsp</b> *<a href='/year/.*?'>(?P<date>.*?)</a><br/><b>Manufacturer:&nbsp</b> *<a href='/manufacturer/.*?'>(?P<developer>.*?)</a><br/><b>Filename:&nbsp;</b>(?P<filename>.*?)<br/><b>")
	cleanTitleRE = regexp.MustCompile("^(.*?)&nbsp.*$")
	scoreRE      = regexp.MustCompile("<b>Score:&nbsp;</b>(.*?) \\(.*? votes\\)<br/>")
	genreRE      = regexp.MustCompile("<b>Category:&nbsp;</b><a .*?>(.*?)</a><br/>")
	playersRE    = regexp.MustCompile("<b>Players:&nbsp;</b>(.*?)<br/>")
	snapRE       = regexp.MustCompile("<img src='/snap/(.*?)\\.png'")
	titleImgRE   = regexp.MustCompile("<img src='/titles/(.*?)\\.png'")
	cabinetRE    = regexp.MustCompile("<img src='/cabinets.small/(.*?)\\.(png|jpg|jpeg)'")
	marqueeRE    = regexp.MustCompile("<img src='/marquees.small/(.*?)\\.(png|jpg|jpeg)'")
	cloneRE      = regexp.MustCompile(`\(clone of: <a href='/game/.*?'>(?P<cloneof>.*?)</a>\)&nbsp;<br/>`)
)

// ErrNotFound is returned when a game is not found.
var ErrNotFound = errors.New("rom not found")

// Game represents a game response from mamedb.
type Game struct {
	ID        string
	Name      string
	Title     string
	Snap      string
	Marquee   string
	Cabinet   string
	Developer string
	Rating    float64
	Players   int64
	Genre     string
	Date      string
	Source    string
	CloneOf   string
}

// GetGame gets a game from mamedb.
func GetGame(ctx context.Context, name string) (*Game, error) {
	var g Game
	g.Source = "mamedb.blu-ferret.co.uk"
	g.ID = name
	req, err := http.NewRequest("GET", path+name, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got %d status", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ilm := infoLineRE.FindSubmatch(body)
	if ilm == nil {
		return nil, fmt.Errorf("ILM Bad HTML")
	}
	gameInfo := ilm[1]
	tm := titleRE.FindSubmatch(gameInfo)
	if tm == nil {
		return nil, fmt.Errorf("TM Bad HTML")
	}
	for i, n := range titleRE.SubexpNames() {
		switch n {
		case "title":
			ctn := cleanTitleRE.FindSubmatch(tm[i])
			if ctn != nil {
				g.Name = string(ctn[1])
			} else {
				g.Name = string(tm[i])
			}
		case "date":
			g.Date = string(tm[i])
		case "developer":
			developer := string(tm[i])
			if developer != "<unknown></unknown>" {
				g.Developer = developer
			}
		}
	}
	if m := genreRE.FindSubmatch(gameInfo); m != nil {
		g.Genre = string(m[1])
	}
	if m := cloneRE.FindSubmatch(gameInfo); m != nil {
		g.CloneOf = string(m[1])
	}
	if m := scoreRE.FindSubmatch(body); m != nil {
		rating, err := strconv.ParseFloat(string(m[1]), 64)
		if err == nil {
			g.Rating = rating
		}
	}
	if m := playersRE.FindSubmatch(gameInfo); m != nil {
		players, err := strconv.ParseInt(string(m[1]), 10, 64)
		if err == nil {
			g.Players = players
		}
	}
	if m := snapRE.FindSubmatch(body); m != nil {
		g.Snap = fmt.Sprintf("%s/snap/%s.png", baseURL, string(m[1]))
	}
	if m := marqueeRE.FindSubmatch(body); m != nil {
		g.Marquee = fmt.Sprintf("%s/marquees/%s.png", baseURL, string(m[1]))
	}
	if m := titleImgRE.FindSubmatch(body); m != nil {
		g.Title = fmt.Sprintf("%s/titles/%s.png", baseURL, string(m[1]))
	}
	if m := cabinetRE.FindSubmatch(body); m != nil {
		g.Cabinet = fmt.Sprintf("%s/cabinets/%s.png", baseURL, string(m[1]))
	}
	return &g, nil
}
