// Package gdb interacts with thegamedb.net's API.
//
// Example:
//  resp, err := gdb.GetGame(GGReq{ID: "5"})
package gdb

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antihax/optional"

	gamesdb "github.com/J-Swift/thegamesdb-swagger-client-go"
)

var apiClient = gamesdb.NewAPIClient(gamesdb.NewConfiguration())

// GGLReq represents a request for a GetGameList command.
// type GGLReq struct {
// 	Name     string
// 	Platform string
// 	Genre    string
// }

// GGLResp represents the response of a GetGameList command.
//type GGLResp struct {
//	XMLName xml.Name
//	Game    []GameTrunc
//	Err     string `xml:",chardata"`
//}

//GGReq is the request to GetGame.
type GGReq struct {
	ID       string
	Name     string
	Platform string
}

// GGResp is the response of the GetGame query.
type GGResp struct {
	Games        []gamesdb.Game
	Boxart       map[string][]gamesdb.GameImage
	ImageBaseURL gamesdb.ImageBaseUrlMeta
}

// GetGame gets the game information from the DB.
func GetGame(ctx context.Context, apikey string, req GGReq) (*GGResp, error) {
	var games gamesdb.GamesByGameId
	var resp *http.Response
	var err error

	fields := "players,publishers,genres,overview,last_updated,rating,platform,coop,youtube,os,processor,ram,hdd,video,sound,alternates"

	if req.ID != "" {
		games, resp, err = apiClient.GamesApi.GamesByGameID(ctx, apikey, req.ID, &gamesdb.GamesByGameIDOpts{Fields: optional.NewString(fields)})
	} else if req.Name != "" {
		games, resp, err = apiClient.GamesApi.GamesByGameName(ctx, apikey, req.ID, &gamesdb.GamesByGameNameOpts{Fields: optional.NewString(fields)})
		// TODO(jpr): handle platform
		// apiClient.GamesApi.GamesByGameName(ctx, apikey, req.Name, gamesdb.GamesByGameNameOpts{FilterPlatform:}
	} else {
		return nil, fmt.Errorf("must provide an ID or Name")
	}

	if err != nil {
		return nil, fmt.Errorf("getting game url:%s, error:%s", resp.Request.URL, err)
	}

	res := &GGResp{
		Games: games.Data.Games,
	}

	images, _, err := apiClient.GamesApi.GamesImages(ctx, apikey, req.ID, nil)
	if err == nil {
		res.ImageBaseURL = images.Data.BaseUrl
		res.Boxart = images.Data.Images
	}

	return res, nil
}

// GetGameList gets the game information from the DB.
// func GetGameList(ctx context.Context, req GGLReq) (*GGLResp, error) {
// 	u, err := url.Parse(gdbURL)
// 	u.Path = gglPath
// 	q := url.Values{}
// 	if req.Name == "" {
// 		return nil, fmt.Errorf("must provide Name")
// 	}
// 	q.Set("name", req.Name)
// 	if req.Platform != "" {
// 		q.Set("platform", req.Platform)
// 	}
// 	if req.Genre != "" {
// 		q.Set("genre", req.Genre)
// 	}
// 	u.RawQuery = q.Encode()
// 	hReq, err := http.NewRequest("GET", u.String(), nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	hReq = hReq.WithContext(ctx)
// 	resp, err := http.DefaultClient.Do(hReq)
// 	if err != nil {
// 		return nil, fmt.Errorf("getting game list url:%s, error:%s", u, err)
// 	}
// 	defer resp.Body.Close()
// 	r := &GGLResp{}
// 	decoder := xml.NewDecoder(resp.Body)
// 	if err := decoder.Decode(r); err != nil {
// 		return nil, err
// 	}
// 	if r.XMLName.Local == "Error" {
// 		return nil, fmt.Errorf("GetGameList error: %s", r.Err)
// 	}
// 	r.Err = ""
// 	return r, nil
// }

// IsUp returns if thegamedb.net is up.
func IsUp(ctx context.Context, apikey string) bool {
	_, resp, err := apiClient.GamesApi.GamesByGameID(ctx, apikey, "1", nil)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	return true
}
