/*Copyright (c) 2014 Alec Lofquist

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.*/

// This code was adapted from https://github.com/Aloshi/EmulationStation/pull/200

package mamedb

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
)

const (
	path = "http://www.mamedb.com/game/"
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
	NotFound     = errors.New("rom not found")
)

type Game struct {
	ID        string
	Name      string
	Art       string
	Developer string
	Rating    float64
	Players   int64
	Genre     string
	Date      string
	Source    string
}

func GetGame(name string, imgPriority []string) (*Game, error) {
	var g Game
	g.Source = "mamedb.com"
	g.ID = name
	resp, err := http.Get(path + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, NotFound
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
	tm := titleRE.FindSubmatch(ilm[1])
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
	gm := genreRE.FindSubmatch(ilm[1])
	if gm != nil {
		g.Genre = string(gm[1])
	}
	rm := scoreRE.FindSubmatch(body)
	if rm != nil {
		rating, err := strconv.ParseFloat(string(rm[1]), 64)
		if err == nil {
			g.Rating = rating
		}
	}
	pm := playersRE.FindSubmatch(ilm[1])
	if pm != nil {
		players, err := strconv.ParseInt(string(pm[1]), 10, 64)
		if err == nil {
			g.Players = players
		}
	}
Loop:
	for _, i := range imgPriority {
		switch i {
		case "s":
			sm := snapRE.FindSubmatch(body)
			if sm != nil {
				g.Art = fmt.Sprintf("http://www.mamedb.com/snap/%s.png", string(sm[1]))
				break Loop
			}
		case "m":
			mm := marqueeRE.FindSubmatch(body)
			if mm != nil {
				g.Art = fmt.Sprintf("http://mamedb.com/marquees/%s.png", string(mm[1]))
				break Loop
			}
		case "t":
			tim := titleImgRE.FindSubmatch(body)
			if tim != nil {
				g.Art = fmt.Sprintf("http://mamedb.com/titles/%s.png", string(tim[1]))
				break Loop
			}
		case "c":
			cm := cabinetRE.FindSubmatch(body)
			if cm != nil {
				g.Art = fmt.Sprintf("http://mamedb.com/cabinets/%s.png", string(cm[1]))
				break Loop
			}
		}
	}
	return &g, nil
}
