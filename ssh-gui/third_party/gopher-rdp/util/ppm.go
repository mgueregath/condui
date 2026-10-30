package util

import (
	"os"
	"strconv"
)

// WritePPM writes top-down RGBA pixel data as a PPM (P6) file.
func WritePPM(path string, pix []byte, w, h int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := "P6\n" + strconv.Itoa(w) + " " + strconv.Itoa(h) + "\n255\n"
	f.WriteString(header)
	row := make([]byte, w*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := (y*w + x) * 4
			row[x*3] = pix[si]
			row[x*3+1] = pix[si+1]
			row[x*3+2] = pix[si+2]
		}
		f.Write(row)
	}
	return nil
}
