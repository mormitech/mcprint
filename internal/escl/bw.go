package escl

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
)

// ToBlackAndWhite turns a scanned JPEG into a pure black and white page (still a JPEG, so the rest of the app is
// unchanged), with Otsu's threshold: the grey level that best separates ink from paper.
func ToBlackAndWhite(data []byte) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	gray := image.NewGray(b)
	var hist [256]int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			gray.SetGray(x, y, g)
			hist[g.Y]++
		}
	}
	t := otsu(hist, b.Dx()*b.Dy())
	for i, v := range gray.Pix {
		if int(v) > t {
			gray.Pix[i] = 255
		} else {
			gray.Pix[i] = 0
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, gray, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func otsu(hist [256]int, total int) int {
	var sum float64
	for i, n := range hist {
		sum += float64(i * n)
	}
	var sumB, wB float64
	best, threshold := -1.0, 127
	for t := 0; t < 256; t++ {
		wB += float64(hist[t])
		if wB == 0 {
			continue
		}
		wF := float64(total) - wB
		if wF == 0 {
			break
		}
		sumB += float64(t * hist[t])
		mB, mF := sumB/wB, (sum-sumB)/wF
		if v := wB * wF * (mB - mF) * (mB - mF); v > best {
			best, threshold = v, t
		}
	}
	return threshold
}
