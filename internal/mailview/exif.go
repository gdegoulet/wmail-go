package mailview

import (
	"encoding/binary"
	"image"
	"image/draw"
)

// exifOrientation lit l'étiquette Orientation (0x0112) d'un JPEG. Elle vaut 1 à 8 ; 0 si absente ou illisible.
// Seul ce champ est lu : le reste de l'EXIF (position, appareil…) est ignoré et ne sort jamais du serveur.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 { // SOI d'un JPEG
		return 0
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 0
		}
		marker := data[i+1]
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD9) {
			i += 2
			continue
		}
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if size < 2 || i+2+size > len(data) {
			return 0
		}
		seg := data[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		if marker == 0xDA { // début de l'image compressée : plus d'en-tête ensuite
			return 0
		}
		i += 2 + size
	}
	return 0
}

// tiffOrientation parcourt l'IFD0 d'un bloc TIFF et renvoie la valeur de l'étiquette Orientation.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	if bo.Uint16(t[2:4]) != 42 {
		return 0
	}
	off := int(bo.Uint32(t[4:8]))
	if off < 8 || off+2 > len(t) {
		return 0
	}
	n := int(bo.Uint16(t[off : off+2]))
	if n > 512 { // en-tête abîmé : on n'insiste pas
		return 0
	}
	for k := range n {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 0
		}
		if bo.Uint16(t[e:e+2]) != 0x0112 {
			continue
		}
		if bo.Uint16(t[e+2:e+4]) != 3 { // SHORT
			return 0
		}
		v := int(bo.Uint16(t[e+8 : e+10]))
		if v >= 1 && v <= 8 {
			return v
		}
		return 0
	}
	return 0
}

// applyOrientation redresse une image selon l'étiquette EXIF (rotations et miroirs des valeurs 2 à 8).
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	swap := orientation >= 5 // 5 à 8 : l'image pivote d'un quart de tour
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	if swap {
		out = image.NewRGBA(image.Rect(0, 0, h, w))
	}
	for y := range h {
		for x := range w {
			var dx, dy int
			switch orientation {
			case 2: // miroir horizontal
				dx, dy = w-1-x, y
			case 3: // demi-tour
				dx, dy = w-1-x, h-1-y
			case 4: // miroir vertical
				dx, dy = x, h-1-y
			case 5: // transposée
				dx, dy = y, x
			case 6: // quart de tour horaire
				dx, dy = h-1-y, x
			case 7: // transposée inverse
				dx, dy = h-1-y, w-1-x
			case 8: // quart de tour antihoraire
				dx, dy = y, w-1-x
			}
			out.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	var _ draw.Image = out
	return out
}
