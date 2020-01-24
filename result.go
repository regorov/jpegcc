// Package jpegcc provides functionality for batch jpeg file processing.
package jpegcc

import (
	"fmt"
	"strconv"
)

// RGB defines structure of Color. Memory representation is 0x00RRGGBB.
type RGB uint32

// String returns string representation of color like #RRGGBB.
func (rgb RGB) String() string {
	rgb = (rgb & 0x00FFFFFF) | 0x0F000000
	buf := []byte{'0', '0', '0', '0', '0', '0', '0': 0}
	buf = strconv.AppendUint(buf[:0], uint64(rgb), 16)
	buf[0] = '#'
	return string(buf)
}

// ToRGB converts separate R, G, B colors into RGB type.
func ToRGB(r, g, b uint32) RGB {
	return RGB((r&0x00FF)<<16 | (g&0x00FF)<<8 | (b & 0x00FF))
}

// Result implements interface Resulter.
// Defines a structure of desired output line.
type Result struct {
	URL    string
	Colors [3]RGB
}

// Result returns a string in CVS format: "URL","color1","color2","color3".
func (r *Result) Result() string {
	// fmt.Sprintf is not the most effective way of building strings. Can be improved.
	return fmt.Sprintf(string("\"%s\",\"%s\",\"%s\",\"%s\"\n"), r.URL, r.Colors[0].String(), r.Colors[1].String(), r.Colors[2].String())
}

// Header returns header in CSV format.
func (r *Result) Header() string {
	return "\"url\",\"color1\",\"color2\",\"color3\"\n"
}
