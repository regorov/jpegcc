package jpegcc

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
)

// CounterPix counts 3 most prevalent colors walking
// through the array of pixels. Works twice faster than DefaultCounter
// but consumes more memory.
type CounterPix struct {
}

// NewCounterPix returns CounterPix instance.
func NewCounterPix() *CounterPix {
	return &CounterPix{}
}

// Count implements interface Counter.
func (cc *CounterPix) Count(pic Imager) (Resulter, error) {

	oimg, err := jpeg.Decode(bytes.NewReader(pic.Bytes()))
	if err != nil {
		return nil, errors.New("image could not be decoded [" + err.Error() + "]")
	}

	bou := oimg.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, bou.Dx(), bou.Dy()))
	draw.Draw(img, img.Bounds(), oimg, bou.Min, draw.Src)

	var (
		// stored as RGB(uint32) instead of [3]byte, because
		// map uses special fast hash algo for uint32.
		color = make(map[RGB]uint32, 10000)
	)

	var r, g, b uint32
	// Pix holds the image's pixels, in R, G, B, A order.
	for i := 0; i < len(img.Pix)/4; i += 4 {
		r, g, b = uint32(img.Pix[i]), uint32(img.Pix[i+1]), uint32(img.Pix[i+2])
		color[ToRGB(r, g, b)]++
	}

	// chosing max 3 colors.
	type maxt struct {
		color RGB
		count uint32
	}

	// if there are 1 pixel only, reproduce the same color 3 times.
	var max = [3]maxt{{ToRGB(r, g, b), 1}, {ToRGB(r, g, b), 1}, {ToRGB(r, g, b), 1}}

	for c, cnt := range color {
		if cnt < max[2].count {
			continue
		}
		if cnt > max[0].count {
			max[2] = max[1]
			max[1] = max[0]
			max[0] = maxt{c, cnt}
			continue
		}
		if cnt > max[1].count {
			max[2] = max[1]
			max[1] = maxt{c, cnt}
			continue
		}
		if cnt > max[2].count {
			max[2] = maxt{c, cnt}
		}
	}

	return &Result{URL: pic.URL(), Colors: [3]RGB{max[0].color, max[1].color, max[2].color}}, nil
}
