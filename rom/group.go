package rom

import (
	"context"

	"github.com/sselph/scraper/ds"
)

// GameOpts represents the options for creating Game information.
type GameOpts struct {
	// AddNotFound instructs the scraper to create a Game even if the game isn't in the sources.
	AddNotFound bool
	// NoPrettyName instructs the scraper to leave the name as the name in the source.
	NoPrettyName bool
	// UseFilename instructs the scraper to use the filename minus extension as the xml name.
	UseFilename bool
	// NoStripUnicode instructs the scraper to not strip out unicode characters.
	NoStripUnicode bool
	// OverviewLen is the max length allowed for a overview. 0 means no limit.
	OverviewLen int
}

// XMLOpts represents the options for creating XML information.
type XMLOpts struct {
	// RomDir is the base directory for scraping rom files.
	RomDir string
	// RomXMLDir is the base directory where roms will be located on the target system.
	RomXMLDir string
	// NestImgDir if true tells the scraper to use the same directory structure of roms for rom images.
	NestImgDir bool
	// ImgDir is the base directory for downloading images.
	ImgDir string
	// ImgXMLDir is the directory where images will be located on the target system.
	ImgXMLDir string
	// ImgPriority is the order or image preference when multiple images are avialable.
	ImgPriority []ds.ImgType
	// ImgSuffix is what will be appened to the end of the rom's name to name the image
	// ie rom.bin with suffix of "-image" results in rom-image.jpg
	ImgSuffix string
	// ThumbOnly tells the scraper to prefer thumbnail size images when available.
	ThumbOnly bool
	// NoDownload tells the scraper to not download images.
	NoDownload bool
	// ImgFormat is the format for the image, currently only "jpg" and "png" are supported.
	ImgFormat string
	// ImgWidth is the max width of images. Anything larger will be resized.
	ImgWidth uint
	// ImgHeight is the max height of images. Anything larger will be resized.
	ImgHeight    uint
	DownloadVid  bool
	VidPriority  []ds.VidType
	VidSuffix    string
	VidDir       string
	VidXMLDir    string
	VidConvert   bool
	DownloadMarq bool
	MarqSuffix   string
	MarqDir      string
	MarqXMLDir   string
	MarqFormat   string
}

type ROMResult struct {
	Rom   *ROM
	Error error
}

type group struct {
	roms []*ROM
}

func NewGroup(roms []*ROM) group {
	return group{
		roms: roms,
	}
}

func (grp *group) GetGames(ctx context.Context, data []ds.DS, opts *GameOpts, onResult chan ROMResult, done chan struct{}) {
	if opts == nil {
		opts = &GameOpts{}
	}

	for _, rom := range grp.roms {
		err := rom.GetGame(ctx, data, opts)

		onResult <- ROMResult{
			Rom:   rom,
			Error: err,
		}
	}

	close(done)
}
