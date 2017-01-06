package ds

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/sselph/scraper/ss"
)

// SS is the source for ScreenScraper
type SS struct {
	HM     *HashMap
	Hasher *Hasher
	Dev    ss.DevInfo
	User   ss.UserInfo
	Lang   []LangType
	Region []RegionType
	Width  int
}

// getID gets the ID from the path.
func (s *SS) getID(p string) (string, error) {
	return s.Hasher.Hash(p)
}

// GetName implements DS
func (s *SS) GetName(p string) string {
	h, err := s.Hasher.Hash(p)
	if err != nil {
		return ""
	}
	name, ok := s.HM.Name(h)
	if !ok {
		return ""
	}
	return name
}

// ssBoxURL gets the URL for BoxArt for the preferred region.
func ssBoxURL(media ss.GameMedia, regions []RegionType, width int) string {
	for _, r := range regions {
		switch r {
		case RegionUS:
			if media.BoxUS != "" {
				return ssImgURL(media.BoxUS, width)
			}
		case RegionEU:
			if media.BoxEU != "" {
				return ssImgURL(media.BoxEU, width)
			}
		case RegionFR:
			if media.BoxFR != "" {
				return ssImgURL(media.BoxFR, width)
			}
		case RegionJP:
			if media.BoxJP != "" {
				return ssImgURL(media.BoxJP, width)
			}
		case RegionXX:
			if media.BoxXX != "" {
				return ssImgURL(media.BoxXX, width)
			}
		}
	}
	return ""
}

// ss3DBoxURL gets the URL for 3D-BoxArt for the preferred region.
func ss3DBoxURL(media ss.GameMedia, regions []RegionType, width int) string {
	for _, r := range regions {
		switch r {
		case RegionUS:
			if media.Box3DUS != "" {
				return ssImgURL(media.Box3DUS, width)
			}
		case RegionEU:
			if media.Box3DEU != "" {
				return ssImgURL(media.Box3DEU, width)
			}
		case RegionFR:
			if media.Box3DFR != "" {
				return ssImgURL(media.Box3DFR, width)
			}
		case RegionJP:
			if media.Box3DJP != "" {
				return ssImgURL(media.Box3DJP, width)
			}
		case RegionXX:
			if media.Box3DXX != "" {
				return ssImgURL(media.Box3DXX, width)
			}
		}
	}
	return ""
}

// ssDate gets the date for the preferred region.
func ssDate(dates ss.GameDates, regions []RegionType) string {
	for _, r := range regions {
		switch r {
		case RegionUS:
			if dates.US != "" {
				return toXMLDate(dates.US)
			}
		case RegionEU:
			if dates.EU != "" {
				return toXMLDate(dates.EU)
			}
		case RegionFR:
			if dates.FR != "" {
				return toXMLDate(dates.FR)
			}
		case RegionJP:
			if dates.JP != "" {
				return toXMLDate(dates.JP)
			}
		case RegionXX:
			if dates.XX != "" {
				return toXMLDate(dates.XX)
			}
		}
	}
	return ""
}

// ssDesc gets the desc for the preferred language.
func ssDesc(desc ss.GameDesc, lang []LangType) string {
	for _, l := range lang {
		switch l {
		case LangEN:
			if desc.EN != "" {
				return desc.EN
			}
		case LangFR:
			if desc.FR != "" {
				return desc.FR
			}
		case LangES:
			if desc.ES != "" {
				return desc.ES
			}
		case LangDE:
			if desc.DE != "" {
				return desc.DE
			}
		case LangPT:
			if desc.PT != "" {
				return desc.PT
			}
		}
	}
	return ""
}

// ssName gets the name for the preferred language, else the original.
func ssName(name ss.GameNames, lang []LangType) string {
	for _, l := range lang {
		switch l {
		case LangEN:
			if name.EN != "" {
				return name.EN
			}
		case LangFR:
			if name.FR != "" {
				return name.FR
			}
		case LangES:
			if name.ES != "" {
				return name.ES
			}
		case LangDE:
			if name.DE != "" {
				return name.DE
			}
		case LangPT:
			if name.PT != "" {
				return name.PT
			}
		}
	}
	return name.Original
}

// ssGenre gets the genre for the preferred language.
func ssGenre(genre ss.GameGenre, lang []LangType) string {
	for _, l := range lang {
		switch l {
		case LangEN:
			if genre.EN != nil {
				return strings.Join(genre.EN, ",")
			}
		case LangFR:
			if genre.FR != nil {
				return strings.Join(genre.FR, ",")
			}
		case LangES:
			if genre.ES != nil {
				return strings.Join(genre.ES, ",")
			}
		case LangDE:
			if genre.DE != nil {
				return strings.Join(genre.DE, ",")
			}
		case LangPT:
			if genre.PT != nil {
				return strings.Join(genre.PT, ",")
			}
		}
	}
	return ""
}

// romRegion extracts the region from the No-Intro name.
func romRegion(n string) RegionType {
	for {
		s := strings.IndexRune(n, '(')
		if s == -1 {
			return RegionUnknown
		}
		e := strings.IndexRune(n[s:], ')')
		if e == -1 {
			return RegionUnknown
		}
		switch n[s : s+e+1] {
		case "(USA)":
			return RegionUS
		case "(Europe)":
			return RegionEU
		case "(Japan)":
			return RegionJP
		case "(France)":
			return RegionFR
		case "(World)":
			return RegionUnknown
		}
		n = n[s+e+1:]
	}
}

// GetGame implements DS
func (s *SS) GetGame(path string) (*Game, error) {
	id, err := s.getID(path)
	if err != nil {
		return nil, err
	}
	req := ss.GameInfoReq{SHA1: id}
	resp, err := ss.GameInfo(s.Dev, s.User, req)
	if err != nil {
		if err == ss.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	game := resp.Game

	region := RegionUnknown
	if name, ok := s.HM.Name(id); ok {
		region = romRegion(name)
	}
	var regions []RegionType
	if region != RegionUnknown {
		regions = append([]RegionType{region}, s.Region...)
	} else {
		regions = s.Region
	}

	ret := NewGame()
	if game.Media.ScreenShot != "" {
		ret.Images[ImgScreen] = ssImgURL(game.Media.ScreenShot, s.Width)
	}
	if imgURL := ssBoxURL(game.Media, regions, s.Width); imgURL != "" {
		ret.Images[ImgBoxart] = imgURL
	}
	if imgURL := ss3DBoxURL(game.Media, regions, s.Width); imgURL != "" {
		ret.Images[ImgBoxart3D] = imgURL
	}
	ret.ID = strconv.Itoa(game.ID)
	ret.Source = "screenscraper.fr"
	ret.GameTitle = game.Names.Original
	ret.Overview = ssDesc(game.Desc, s.Lang)
	game.Rating = strings.TrimSuffix(game.Rating, "/20")
	if r, err := strconv.ParseFloat(game.Rating, 64); err == nil {
		ret.Rating = r / 20.0
	}
	ret.Developer = game.Developer
	ret.Publisher = game.Publisher
	ret.Genre = ssGenre(game.Genre, s.Lang)
	ret.ReleaseDate = ssDate(game.Dates, s.Region)
	if strings.ContainsRune(game.Players, '-') {
		x := strings.Split(game.Players, "-")
		game.Players = x[len(x)-1]
	}
	p, err := strconv.ParseInt(strings.TrimRight(game.Players, "+"), 10, 32)
	if err == nil {
		ret.Players = p
	}
	if ret.Overview == "" || (ret.ReleaseDate == "" && ret.Developer == "" && ret.Publisher == "" && ret.Images[ImgBoxart] == "" && ret.Images[ImgScreen] == "") {
		return nil, ErrNotFound
	}
	return ret, nil
}

// ssImgURL parses the URL and adds the maxwidth.
func ssImgURL(img string, width int) string {
	if width <= 0 {
		return img
	}
	u, err := url.Parse(img)
	if err != nil {
		return img
	}
	v := u.Query()
	v.Set("maxwidth", strconv.Itoa(width))
	u.RawQuery = v.Encode()
	return u.String()
}
