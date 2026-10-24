package mailview

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"  // décodeur GIF
	_ "image/jpeg" // décodeur JPEG
	"image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // décodeur WebP
)

// maxThumbPixels borne la taille décodée d'une image : au-delà, aucune miniature n'est produite
// (une petite image compressée peut annoncer des dimensions énormes).
const maxThumbPixels = 40_000_000

// Thumbnail renvoie une miniature carrée (recadrée au centre) de côté size, en PNG, sous forme d'URI data:.
// Elle ne traite que PNG, JPEG, GIF et WebP : les autres formats (SVG compris) ne donnent pas de miniature.
func Thumbnail(data []byte, size int) (string, bool) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxThumbPixels {
		return "", false
	}
	switch format {
	case "png", "jpeg", "gif", "webp":
	default:
		return "", false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", false
	}
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src) // fond blanc sous la transparence
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, image.Rect(x0, y0, x0+side, y0+side), xdraw.Over, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return "", false
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), true
}

// IsImage indique si la pièce jointe est une image dont on peut produire une miniature.
func (a Attachment) IsImage() bool {
	switch a.ContentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	}
	return false
}

// Data renvoie le contenu de la pièce jointe.
func (a Attachment) Data() []byte { return a.data }
