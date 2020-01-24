package jpegcc

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
)

// CounterDefault counts 3 most prevalent colors, getting RGBA color of
// each pixel one by one.
type CounterDefault struct {
}

// NewCounterDefault creates and returns CounterPix instance.
func NewCounterDefault() *CounterDefault {
	return &CounterDefault{}
}

// Count implements interface Counter.
func (cc *CounterDefault) Count(pic Imager) (Resulter, error) {

	oimg, err := jpeg.Decode(bytes.NewReader(pic.Bytes()))
	if err != nil {
		return nil, errors.New("image could not be decoded [" + err.Error() + "]")
	}

	img, ok := oimg.(*image.YCbCr)
	if !ok {
		return nil, errors.New("image could not be converted to YCbCr")
	}

	bou := img.Bounds()
	width, height := bou.Max.X, bou.Max.Y
	var (
		color = make(map[RGB]uint32, 20000)
	)

	var r, g, b uint32
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ = img.YCbCrAt(x, y).RGBA()
			color[ToRGB(r, g, b)]++
		}
	}

	// choice of 3 max colors.
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
