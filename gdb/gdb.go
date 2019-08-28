// Package gdb interacts with thegamedb.net's API.
package gdb

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/antihax/optional"

	gamesdb "github.com/J-Swift/thegamesdb-swagger-client-go"
)

var apiClient = gamesdb.NewAPIClient(gamesdb.NewConfiguration())

// Publishers

type publishers struct {
	mux        sync.Mutex
	publishers *map[string]gamesdb.Publisher
}

var publishersCache = publishers{}

func getPublishers(ctx context.Context, apikey string) map[string]gamesdb.Publisher {
	pubs, resp, err := apiClient.PublishersApi.Publishers(ctx, apikey)

	if err != nil || resp.StatusCode != 200 {
		return make(map[string]gamesdb.Publisher)
	}

	return pubs.Data.Publishers
}

func getCachedPublishers(ctx context.Context, apikey string) map[string]gamesdb.Publisher {
	publishers := publishersCache.publishers
	if publishers != nil {
		return *publishers
	}

	publishersCache.mux.Lock()
	defer publishersCache.mux.Unlock()

	publishers = publishersCache.publishers
	if publishers == nil {
		apiPublishers := getPublishers(ctx, apikey)
		publishers = &apiPublishers
		publishersCache.publishers = publishers
	}

	return *publishers
}

func parsePublishers(ctx context.Context, apikey string, apiGame gamesdb.Game) []ParsedPublisher {
	allPublishers := getCachedPublishers(ctx, apikey)

	publishers := []ParsedPublisher{}
	for _, publisherID := range apiGame.Publishers {
		if apiPublisher, ok := allPublishers[strconv.Itoa(int(publisherID))]; ok {
			publishers = append(publishers, toParsedPublisher(apiPublisher))
		}
	}
	return publishers
}

// Developers

type developers struct {
	mux        sync.Mutex
	developers *map[string]gamesdb.Developer
}

var developersCache = developers{}

func getDevelopers(ctx context.Context, apikey string) map[string]gamesdb.Developer {
	pubs, resp, err := apiClient.DevelopersApi.Developers(ctx, apikey)

	if err != nil || resp.StatusCode != 200 {
		return make(map[string]gamesdb.Developer)
	}

	return pubs.Data.Developers
}

func getCachedDevelopers(ctx context.Context, apikey string) map[string]gamesdb.Developer {
	developers := developersCache.developers
	if developers != nil {
		return *developers
	}

	developersCache.mux.Lock()
	defer developersCache.mux.Unlock()

	developers = developersCache.developers
	if developers == nil {
		apiDevelopers := getDevelopers(ctx, apikey)
		developers = &apiDevelopers
		developersCache.developers = developers
	}

	return *developers
}

func parseDevelopers(ctx context.Context, apikey string, apiGame gamesdb.Game) []ParsedDeveloper {
	allDevelopers := getCachedDevelopers(ctx, apikey)

	developers := []ParsedDeveloper{}
	for _, developerID := range apiGame.Developers {
		if apiDeveloper, ok := allDevelopers[strconv.Itoa(int(developerID))]; ok {
			developers = append(developers, toParsedDeveloper(apiDeveloper))
		}
	}
	return developers
}

// Genres

type genres struct {
	mux    sync.Mutex
	genres *map[string]gamesdb.Genre
}

var genresCache = genres{}

func getGenres(ctx context.Context, apikey string) map[string]gamesdb.Genre {
	pubs, resp, err := apiClient.GenresApi.Genres(ctx, apikey)

	if err != nil || resp.StatusCode != 200 {
		return make(map[string]gamesdb.Genre)
	}

	return pubs.Data.Genres
}

func getCachedGenres(ctx context.Context, apikey string) map[string]gamesdb.Genre {
	genres := genresCache.genres
	if genres != nil {
		return *genres
	}

	genresCache.mux.Lock()
	defer genresCache.mux.Unlock()

	genres = genresCache.genres
	if genres == nil {
		apiGenres := getGenres(ctx, apikey)
		genres = &apiGenres
		genresCache.genres = genres
	}

	return *genres
}

func parseGenres(ctx context.Context, apikey string, apiGame gamesdb.Game) []ParsedGenre {
	allGenres := getCachedGenres(ctx, apikey)

	genres := []ParsedGenre{}
	for _, genreID := range apiGame.Genres {
		if apiGenre, ok := allGenres[strconv.Itoa(int(genreID))]; ok {
			genres = append(genres, toParsedGenre(apiGenre))
		}
	}
	return genres
}

// ParsedDeveloper is a normalized GamesDB Developer
type ParsedDeveloper struct {
	ID   int
	Name string
}

func toParsedDeveloper(apiDeveloper gamesdb.Developer) ParsedDeveloper {
	return ParsedDeveloper{
		ID:   int(apiDeveloper.Id),
		Name: apiDeveloper.Name,
	}
}

// ParsedGenre is a normalized GamesDB Genre
type ParsedGenre struct {
	ID   int
	Name string
}

func toParsedGenre(apiGenre gamesdb.Genre) ParsedGenre {
	return ParsedGenre{
		ID:   int(apiGenre.Id),
		Name: apiGenre.Name,
	}
}

// ParsedPublisher is a normalized GamesDB Publisher
type ParsedPublisher struct {
	ID   int
	Name string
}

func toParsedPublisher(apiPublisher gamesdb.Publisher) ParsedPublisher {
	return ParsedPublisher{
		ID:   int(apiPublisher.Id),
		Name: apiPublisher.Name,
	}
}

// ParsedGameImage is a normalized GamesDB GameImage
type ParsedGameImage struct {
	ID       int
	Type     string
	Side     string
	Filename string
}

func toParsedGameImage(apiGameImage gamesdb.GameImage) ParsedGameImage {
	return ParsedGameImage{
		ID:       int(apiGameImage.Id),
		Type:     apiGameImage.Type,
		Side:     apiGameImage.Side,
		Filename: apiGameImage.Filename,
	}
}

// ParsedImageSizeBaseUrls is a normalized GamesDB ImageBaseUrlMeta
type ParsedImageSizeBaseUrls struct {
	Original string
	Thumb    string
}

func toParsedImageSizeBaseUrls(apiBaseURLMeta gamesdb.ImageBaseUrlMeta) ParsedImageSizeBaseUrls {
	return ParsedImageSizeBaseUrls{
		Original: apiBaseURLMeta.Original,
		Thumb:    apiBaseURLMeta.Thumb,
	}
}

// ParsedGame is  a normalized GamesDB Game
type ParsedGame struct {
	ID          int
	Name        string
	ReleaseDate string
	//Platform    int
	Players    int
	Overview   string
	Developers []ParsedDeveloper
	Genres     []ParsedGenre
	Publishers []ParsedPublisher

	Images        []ParsedGameImage
	ImageBaseUrls ParsedImageSizeBaseUrls
}

func parseImages(apiImages []gamesdb.GameImage) []ParsedGameImage {
	parsedImages := make([]ParsedGameImage, len(apiImages))

	for idx, apiImage := range apiImages {
		parsedImages[idx] = toParsedGameImage(apiImage)
	}

	return parsedImages
}

type GetGameResult struct {
	Game  *ParsedGame
	Error error
}

func replicatedError(err error, times int) []GetGameResult {
	results := make([]GetGameResult, times)

	for i := 0; i < times; i++ {
		results[i] = GetGameResult{
			Error: err,
		}
	}

	return results
}

// GetGames gets the game information from the DB.
func GetGames(ctx context.Context, apikey string, gameIDs []string) []GetGameResult {
	var games gamesdb.GamesByGameId
	var resp *http.Response
	var err error

	results := []GetGameResult{}

	// TODO(jpr): remove unneeded fields
	//fields := "players,publishers,genres,overview,last_updated,rating,platform,coop,youtube,os,processor,ram,hdd,video,sound,alternates"
	fields := "players,publishers,genres,overview,platform"

	if len(gameIDs) == 0 {
		return results
	}

	joinedIds := strings.Join(gameIDs, ",")

	games, resp, err = apiClient.GamesApi.GamesByGameID(ctx, apikey, joinedIds, &gamesdb.GamesByGameIDOpts{Page: optional.NewInt32(1), Fields: optional.NewString(fields)})

	if err != nil {
		return replicatedError(fmt.Errorf("getting game url:%s, error:%s", resp.Request.URL, err), len(gameIDs))
	}
	if len(games.Data.Games) == 0 {
		return replicatedError(fmt.Errorf("game not found"), len(gameIDs))
	}

	mappedGames := make(map[string]gamesdb.Game)
	for _, game := range games.Data.Games {
		mappedGames[strconv.Itoa(int(game.Id))] = game
	}

	var currentPage int32 = 1
	for {
		if games.Pages.Next == "" || err != nil {
			break
		}
		for _, game := range games.Data.Games {
			mappedGames[strconv.Itoa(int(game.Id))] = game
		}

		currentPage++
		games, _, err = apiClient.GamesApi.GamesByGameID(ctx, apikey, joinedIds, &gamesdb.GamesByGameIDOpts{Page: optional.NewInt32(currentPage), Fields: optional.NewString(fields)})
	}

	images, _, imageErr := apiClient.GamesApi.GamesImages(ctx, apikey, joinedIds, &gamesdb.GamesImagesOpts{Page: optional.NewInt32(1)})

	mappedImages := make(map[string][]gamesdb.GameImage)
	for key, imageInfo := range images.Data.Images {
		mappedImages[key] = imageInfo
	}

	currentPage = 1
	for {
		if images.Pages.Next == "" || imageErr != nil {
			break
		}
		for key, imageInfo := range images.Data.Images {
			mappedImages[key] = imageInfo
		}

		currentPage++
		images, _, imageErr = apiClient.GamesApi.GamesImages(ctx, apikey, joinedIds, &gamesdb.GamesImagesOpts{Page: optional.NewInt32(currentPage)})
	}

	for _, requestedID := range gameIDs {
		apiGame, found := mappedGames[requestedID]
		if !found {
			results = append(results, GetGameResult{
				Error: fmt.Errorf("game not found"),
			})
			continue
		}

		res := &ParsedGame{
			ID:          int(apiGame.Id),
			Name:        apiGame.GameTitle,
			ReleaseDate: apiGame.ReleaseDate,
			Players:     int(apiGame.Players),
			Overview:    apiGame.Overview,
		}

		res.Genres = parseGenres(ctx, apikey, apiGame)
		res.Developers = parseDevelopers(ctx, apikey, apiGame)
		res.Publishers = parsePublishers(ctx, apikey, apiGame)

		if apiImages, found := mappedImages[requestedID]; found {
			res.ImageBaseUrls = toParsedImageSizeBaseUrls(images.Data.BaseUrl)
			res.Images = parseImages(apiImages)
		}

		results = append(results, GetGameResult{
			Game: res,
		})
	}

	return results
}

// IsUp returns if thegamedb.net is up.
func IsUp(ctx context.Context, apikey string) bool {
	_, resp, err := apiClient.GamesApi.GamesByGameID(ctx, apikey, "1", nil)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	return true
}
