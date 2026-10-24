package escl

import (
	"fmt"
	"strings"
)

// Options is what the user picks for a scan.
type Options struct {
	Source string `json:"source"` // "platen" or "feeder"
	Duplex bool   `json:"duplex"`
	DPI    int    `json:"dpi"`
	Color  string `json:"color"` // "color", "gray" or "bw"
	Paper  string `json:"paper"` // a4, a5, letter, legal or max
}

// Resolved is Options checked against a scanner, with the page size in 1/300 inch.
type Resolved struct {
	Options
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Choices is what the UI may offer for one source.
type Choices struct {
	Sources   []string `json:"sources"`
	CanDuplex bool     `json:"canDuplex"`
	Colors    []string `json:"colors"`
	DPIs      []int    `json:"dpis"`
	Papers    []string `json:"papers"`
	DefaultDP int      `json:"defaultDpi"`
}

var colorModes = map[string]string{"color": "RGB24", "gray": "Grayscale8", "bw": "BlackAndWhite1"}
var colorOrder = []string{"color", "gray", "bw"}
var sources = map[string]string{"platen": "Platen", "feeder": "Feeder"}

// PaperSizes are in 1/300 inch (width, height).
var PaperSizes = map[string][2]int{"a4": {2480, 3507}, "a5": {1748, 2480}, "letter": {2550, 3300}, "legal": {2550, 4200}}
var paperOrder = []string{"a4", "a5", "letter", "legal", "max"}
var standardDPIs = []int{75, 100, 150, 200, 300, 400, 600, 1200, 2400}

func (r Resolutions) supports(dpi int) bool {
	if len(r.Values) == 0 && r.Max == 0 {
		return true // unknown: let the scanner decide
	}
	for _, v := range r.Values {
		if v == dpi {
			return true
		}
	}
	return r.Max > 0 && dpi >= r.Min && dpi <= r.Max && (dpi-r.Min)%max(r.Step, 1) == 0
}

func (r Resolutions) choices() []int {
	if len(r.Values) > 0 {
		return append([]int{}, r.Values...)
	}
	var out []int
	for _, d := range standardDPIs {
		if r.supports(d) && r.Max > 0 {
			out = append(out, d)
		}
	}
	return out
}

func (r Resolutions) defaultDPI() int {
	if r.supports(300) {
		return 300
	}
	opts := r.choices()
	if len(opts) == 0 {
		return 300
	}
	best := opts[0]
	for _, v := range opts {
		if abs(v-300) < abs(best-300) {
			best = v
		}
	}
	return best
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func (c *Caps) hasFeeder() bool { return c.Simplex != nil || c.Duplex != nil }

// input picks the capabilities of the chosen source.
func (c *Caps) input(source string, duplex bool) (*Input, error) {
	switch source {
	case "platen":
		if c.Platen == nil {
			return nil, fmt.Errorf("this scanner has no flatbed")
		}
		if duplex {
			return nil, fmt.Errorf("two-sided scanning is only available from the feeder")
		}
		return c.Platen, nil
	case "feeder":
		if !c.hasFeeder() {
			return nil, fmt.Errorf("this scanner has no document feeder")
		}
		if duplex {
			if c.Duplex == nil {
				return nil, fmt.Errorf("this scanner cannot scan both sides of a sheet")
			}
			return c.Duplex, nil
		}
		if c.Simplex != nil {
			return c.Simplex, nil
		}
		return c.Duplex, nil
	}
	return nil, fmt.Errorf("unknown source %q", source)
}

// DefaultSource is the feeder when there is one, else the flatbed.
func (c *Caps) DefaultSource() string {
	if c.hasFeeder() {
		return "feeder"
	}
	return "platen"
}

// Choices lists what can be picked for a source.
func (c *Caps) Choices(source string, duplex bool) (Choices, error) {
	ch := Choices{Sources: []string{}, Colors: []string{}, DPIs: []int{}, Papers: paperOrder}
	if c.Platen != nil {
		ch.Sources = append(ch.Sources, "platen")
	}
	if c.hasFeeder() {
		ch.Sources = append(ch.Sources, "feeder")
	}
	ch.CanDuplex = c.Duplex != nil
	if source == "" {
		source = c.DefaultSource()
	}
	in, err := c.input(source, duplex)
	if err != nil {
		return ch, err
	}
	for _, name := range colorOrder {
		if len(in.ColorModes) == 0 || contains(in.ColorModes, colorModes[name]) {
			ch.Colors = append(ch.Colors, name)
		}
	}
	ch.DPIs = in.Resolutions.choices()
	ch.DefaultDP = in.Resolutions.defaultDPI()
	return ch, nil
}

func contains(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}

// Resolve checks the options against the scanner (zero values mean "default") and returns what to send.
func (c *Caps) Resolve(o Options) (Resolved, error) {
	if o.Source == "" {
		o.Source = c.DefaultSource()
	}
	in, err := c.input(o.Source, o.Duplex)
	if err != nil {
		return Resolved{}, err
	}
	if o.DPI == 0 {
		o.DPI = in.Resolutions.defaultDPI()
	} else if !in.Resolutions.supports(o.DPI) {
		return Resolved{}, fmt.Errorf("unsupported resolution %d dpi (supported: %v)", o.DPI, in.Resolutions.choices())
	}
	if o.Color == "" {
		for _, name := range colorOrder {
			if len(in.ColorModes) == 0 || contains(in.ColorModes, colorModes[name]) {
				o.Color = name
				break
			}
		}
	}
	mode, ok := colorModes[o.Color]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown colour mode %q", o.Color)
	}
	if len(in.ColorModes) > 0 && !contains(in.ColorModes, mode) {
		return Resolved{}, fmt.Errorf("this scanner does not offer %q (supported: %s)", o.Color, strings.Join(in.ColorModes, ", "))
	}
	if len(in.Formats) > 0 && !contains(in.Formats, "image/jpeg") {
		return Resolved{}, fmt.Errorf("this scanner cannot deliver JPEG images")
	}
	if o.Paper == "" {
		o.Paper = "a4"
	}
	size, ok := PaperSizes[o.Paper]
	if o.Paper == "max" {
		size, ok = PaperSizes["a4"], true
		if in.MaxWidth > 0 && in.MaxHeight > 0 {
			size = [2]int{in.MaxWidth, in.MaxHeight}
		}
	}
	if !ok {
		return Resolved{}, fmt.Errorf("unknown paper size %q", o.Paper)
	}
	w, h := size[0], size[1]
	if in.MaxWidth > 0 {
		w = min(w, in.MaxWidth)
	}
	if in.MaxHeight > 0 {
		h = min(h, in.MaxHeight)
	}
	return Resolved{Options: o, Width: w, Height: h}, nil
}

// SettingsXML is the body of the ScanJobs request. Element order and content follow sane-airscan.
func SettingsXML(r Resolved) string {
	duplex := ""
	// The Brother MFC-L2710DN only feeds from the ADF when duplex is explicitly given.
	if r.Source == "feeder" {
		duplex = fmt.Sprintf("\n  <scan:Duplex>%t</scan:Duplex>", r.Duplex)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<scan:ScanSettings xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03" xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm">
  <pwg:Version>2.0</pwg:Version>
  <scan:Intent>Document</scan:Intent>
  <pwg:ScanRegions>
    <pwg:ScanRegion>
      <pwg:ContentRegionUnits>escl:ThreeHundredthsOfInches</pwg:ContentRegionUnits>
      <pwg:XOffset>0</pwg:XOffset>
      <pwg:YOffset>0</pwg:YOffset>
      <pwg:Width>%d</pwg:Width>
      <pwg:Height>%d</pwg:Height>
    </pwg:ScanRegion>
  </pwg:ScanRegions>
  <pwg:InputSource>%s</pwg:InputSource>
  <scan:ColorMode>%s</scan:ColorMode>
  <pwg:DocumentFormat>image/jpeg</pwg:DocumentFormat>
  <scan:DocumentFormatExt>image/jpeg</scan:DocumentFormatExt>
  <scan:XResolution>%d</scan:XResolution>
  <scan:YResolution>%d</scan:YResolution>%s
</scan:ScanSettings>`, r.Width, r.Height, sources[r.Source], colorModes[r.Color], r.DPI, r.DPI, duplex)
}
