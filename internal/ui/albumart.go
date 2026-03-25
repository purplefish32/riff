package ui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"net/http"
	"strings"
	"sync"
)

// artCache caches rendered art strings by coverID+size key.
var (
	artCache   = make(map[string]string)
	artCacheMu sync.Mutex
)

// fetchAlbumArt downloads and renders album art as unicode half-blocks.
// Returns a multi-line string of width cols and height rows characters.
// Each character row represents 2 pixel rows using the upper half block (▀).
// Returns empty string on error or when noColor is true.
func fetchAlbumArt(coverID string, cols, rows int) string {
	if noColor || coverID == "" {
		return ""
	}

	key := fmt.Sprintf("%s/%d/%d", coverID, cols, rows)
	artCacheMu.Lock()
	if cached, ok := artCache[key]; ok {
		artCacheMu.Unlock()
		return cached
	}
	artCacheMu.Unlock()

	urlCover := strings.ReplaceAll(coverID, "-", "/")
	imgURL := fmt.Sprintf("https://resources.tidal.com/images/%s/80x80.jpg", urlCover)

	resp, err := http.Get(imgURL) //nolint:noctx
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return ""
	}

	result := renderImageHalfBlock(img, cols, rows)

	artCacheMu.Lock()
	artCache[key] = result
	artCacheMu.Unlock()

	return result
}

// samplePixel samples the source image using nearest-neighbor at the given
// destination coordinates, scaling from (srcW x srcH) to (dstW x dstH).
func samplePixel(img image.Image, x, y, srcW, srcH, dstW, dstH int) color.Color {
	sx := x * srcW / dstW
	sy := y * srcH / dstH
	return img.At(sx, sy)
}

// renderImageHalfBlock renders img as a block of unicode half-block characters
// using 24-bit ANSI color escape codes. Each character cell covers 2 vertical
// pixel rows: the upper half block (▀) is colored with the top pixel as
// foreground and the bottom pixel as background.
func renderImageHalfBlock(img image.Image, cols, rows int) string {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	pixelRows := rows * 2 // 2 pixel rows per character row

	var sb strings.Builder
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			topY := row * 2
			botY := row*2 + 1

			top := samplePixel(img, col, topY, srcW, srcH, cols, pixelRows)
			bot := samplePixel(img, col, botY, srcW, srcH, cols, pixelRows)

			tr, tg, tb, _ := top.RGBA()
			br, bg, bb, _ := bot.RGBA()

			// RGBA returns 16-bit values (0–65535); shift right 8 to get 8-bit (0–255).
			sb.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
				tr>>8, tg>>8, tb>>8,
				br>>8, bg>>8, bb>>8))
		}
		sb.WriteString("\033[0m")
		if row < rows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
