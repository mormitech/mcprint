// Package pdf writes a PDF with one JPEG per page, using the standard library only.
package pdf

import (
	"bytes"
	"fmt"
	"image/color"
	"image/jpeg"
)

// Page is one scanned page. DPI sets the physical size of the page.
type Page struct {
	JPEG []byte
	DPI  int
}

// Build returns a PDF document with the pages in order. The JPEG data is embedded as it is (no recompression).
func Build(pages []Page) ([]byte, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages")
	}
	var buf bytes.Buffer
	var offsets []int
	obj := func(n int, body string) {
		for len(offsets) < n {
			offsets = append(offsets, 0)
		}
		offsets[n-1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	kids := ""
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", 3+3*i)
	}
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	obj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, len(pages)))

	for i, p := range pages {
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(p.JPEG))
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i+1, err)
		}
		dpi := p.DPI
		if dpi <= 0 {
			dpi = 300
		}
		w, h := float64(cfg.Width)*72/float64(dpi), float64(cfg.Height)*72/float64(dpi)
		space := "/DeviceRGB"
		if cfg.ColorModel == color.GrayModel {
			space = "/DeviceGray"
		}
		page, content, img := 3+3*i, 4+3*i, 5+3*i
		obj(page, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>", w, h, img, content))
		stream := fmt.Sprintf("q %.2f 0 0 %.2f 0 0 cm /Im0 Do Q", w, h)
		obj(content, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))

		for len(offsets) < img {
			offsets = append(offsets, 0)
		}
		offsets[img-1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
			img, cfg.Width, cfg.Height, space, len(p.JPEG))
		buf.Write(p.JPEG)
		buf.WriteString("\nendstream\nendobj\n")
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes(), nil
}
