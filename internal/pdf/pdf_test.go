package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

func testJPEG(t *testing.T, gray bool) []byte {
	var img image.Image = image.NewRGBA(image.Rect(0, 0, 30, 20))
	if gray {
		g := image.NewGray(image.Rect(0, 0, 30, 20))
		g.SetGray(1, 1, color.Gray{200})
		img = g
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestBuild(t *testing.T) {
	out, err := Build([]Page{{JPEG: testJPEG(t, false), DPI: 300}, {JPEG: testJPEG(t, true), DPI: 150}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"%PDF-1.4", "/Count 2", "/DeviceRGB", "/DeviceGray", "/MediaBox [0 0 7.20 4.80]", "/MediaBox [0 0 14.40 9.60]", "startxref", "%%EOF"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestBuildErrors(t *testing.T) {
	if _, err := Build(nil); err == nil {
		t.Error("no pages should fail")
	}
	if _, err := Build([]Page{{JPEG: []byte("nope")}}); err == nil {
		t.Error("bad jpeg should fail")
	}
}
