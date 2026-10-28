package main

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"mormiprint/internal/escl"
)

// Demo mode (--mock): made-up scanners and already scanned pages, for screenshots. Nothing real is searched,
// scanned, saved or read: the cache and settings folders are a temporary one that is removed on exit.

//go:embed mock/pages/*.jpg
var mockPageFiles embed.FS

type mockState struct {
	registry *escl.Registry
	caps     map[string]*escl.Caps
	docs     [][]byte          // the made-up documents, in the order they are "scanned"
	icons    map[string]string // the pictures some made-up scanners offer of themselves; the others offer none

	mu   sync.Mutex
	next int
}

const (
	mockAlder   = "7c1d9e52-3b84-4f0a-9d6e-2a51c8b4f001"
	mockKestrel = "7c1d9e52-3b84-4f0a-9d6e-2a51c8b4f002"
	mockHeron   = "7c1d9e52-3b84-4f0a-9d6e-2a51c8b4f003"
)

// mockCapsXML writes a ScannerCapabilities document. A dpi range is given as min, max, step; otherwise the list is used.
func mockCapsXML(make, model, uuid string, platen bool, simplex, duplex bool, feederColors []string, dpis []int, rng [3]int, feederDpis []int) string {
	colors := func(list []string) string {
		var b strings.Builder
		for _, c := range list {
			fmt.Fprintf(&b, "<scan:ColorMode>%s</scan:ColorMode>", c)
		}
		return b.String()
	}
	res := func(list []int, r [3]int) string {
		if r[1] > 0 {
			return fmt.Sprintf("<scan:ResolutionRange><scan:XResolutionRange><scan:Min>%d</scan:Min><scan:Max>%d</scan:Max><scan:Step>%d</scan:Step></scan:XResolutionRange></scan:ResolutionRange>", r[0], r[1], r[2])
		}
		var b strings.Builder
		b.WriteString("<scan:DiscreteResolutions>")
		for _, d := range list {
			fmt.Fprintf(&b, "<scan:DiscreteResolution><scan:XResolution>%d</scan:XResolution><scan:YResolution>%d</scan:YResolution></scan:DiscreteResolution>", d, d)
		}
		b.WriteString("</scan:DiscreteResolutions>")
		return b.String()
	}
	input := func(tag string, cs []string, dp []int, r [3]int, maxH int) string {
		return fmt.Sprintf(`<scan:%s><scan:MinWidth>64</scan:MinWidth><scan:MaxWidth>2550</scan:MaxWidth><scan:MinHeight>64</scan:MinHeight><scan:MaxHeight>%d</scan:MaxHeight><scan:SettingProfiles><scan:SettingProfile><scan:ColorModes>%s</scan:ColorModes><scan:DocumentFormats><pwg:DocumentFormat>image/jpeg</pwg:DocumentFormat></scan:DocumentFormats><scan:SupportedResolutions>%s</scan:SupportedResolutions></scan:SettingProfile></scan:SettingProfiles></scan:%s>`, tag, maxH, colors(cs), res(dp, r), tag)
	}
	all := []string{"BlackAndWhite1", "Grayscale8", "RGB24"}
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?><scan:ScannerCapabilities xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03" xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm"><pwg:Version>2.63</pwg:Version><pwg:MakeAndModel>%s</pwg:MakeAndModel><scan:Manufacturer>%s</scan:Manufacturer><scan:UUID>%s</scan:UUID>`, model, make, uuid)
	if platen {
		fmt.Fprintf(&b, "<scan:Platen>%s</scan:Platen>", input("PlatenInputCaps", all, dpis, rng, 3507))
	}
	if simplex || duplex {
		b.WriteString("<scan:Adf>")
		if simplex {
			b.WriteString(input("AdfSimplexInputCaps", feederColors, feederDpis, [3]int{}, 4200))
		}
		if duplex {
			b.WriteString(input("AdfDuplexInputCaps", feederColors, feederDpis, [3]int{}, 4200))
		}
		b.WriteString("<scan:FeederCapacity>50</scan:FeederCapacity></scan:Adf>")
	}
	b.WriteString("</scan:ScannerCapabilities>")
	return b.String()
}

func newMock() (*mockState, error) {
	m := &mockState{caps: map[string]*escl.Caps{}}
	files, err := mockPageFiles.ReadDir("mock/pages")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		data, err := mockPageFiles.ReadFile("mock/pages/" + n)
		if err != nil {
			return nil, err
		}
		m.docs = append(m.docs, data)
	}

	rgbGray := []string{"Grayscale8", "RGB24"}
	for guid, xml := range map[string]string{
		mockAlder:   mockCapsXML("Alder", "Alder DocuScan 5400", mockAlder, true, true, true, []string{"BlackAndWhite1", "Grayscale8", "RGB24"}, []int{100, 200, 300, 600}, [3]int{}, []int{100, 150, 200, 300, 600}),
		mockKestrel: mockCapsXML("Kestrel", "Kestrel MFP 220 series", mockKestrel, true, true, false, rgbGray, nil, [3]int{100, 600, 50}, []int{150, 200, 300}),
		mockHeron:   mockCapsXML("Heron", "Heron Scan Station", mockHeron, true, false, false, rgbGray, []int{200, 300}, [3]int{}, nil),
	} {
		caps, err := escl.ParseCaps([]byte(xml))
		if err != nil {
			return nil, err
		}
		m.caps[guid] = caps
	}

	m.icons = map[string]string{mockAlder: mockIcon("#2f6fdb"), mockKestrel: mockIcon("#1f8a5b")}

	now := time.Now()
	m.registry = escl.NewMockRegistry([]escl.Device{
		{GUID: mockAlder, Name: "Alder DocuScan 5400", Model: "Alder DocuScan 5400", Host: "192.168.1.40", Port: 80, Scheme: "http", Root: "eSCL", LastSeen: now, Online: true},
		{GUID: mockKestrel, Name: "Kestrel MFP 220 series", Model: "Kestrel MFP 220 series", Host: "192.168.1.52", Port: 80, Scheme: "http", Root: "eSCL", LastSeen: now, Online: true},
		{GUID: mockHeron, Name: "Heron Scan Station", Model: "Heron Scan Station", Host: "192.168.1.77", Port: 80, Scheme: "http", Root: "eSCL", LastSeen: now.Add(-72 * time.Hour), Online: false},
	})
	return m, nil
}

// enableMock switches the app to demo mode: temporary folders, made-up scanners and four pages already scanned.
func (a *App) enableMock() error {
	m, err := newMock()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "mormiprint-demo-")
	if err != nil {
		return err
	}
	a.cacheDir, a.confDir, a.mock = dir, dir, m
	if a.mode == "" {
		for range 4 {
			a.addPage(m.nextDoc(), 150)
		}
	}
	return nil
}

func (m *mockState) nextDoc() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.docs[m.next%len(m.docs)]
	m.next++
	return d
}

// scan "scans" a few pages one after the other, like a feeder would: one for the flatbed, two from the feeder,
// four for both sides.
func (m *mockState) scan(ctx context.Context, r escl.Resolved, onPage func([]byte)) (int, error) {
	n := 1
	if r.Source == "feeder" {
		n = 2
		if r.Duplex {
			n = 4
		}
	}
	for i := 0; i < n; i++ {
		select {
		case <-ctx.Done():
			return i, ctx.Err()
		case <-time.After(900 * time.Millisecond):
		}
		onPage(m.nextDoc())
	}
	return n, nil
}

// mockIcon is a small printer picture in the given colour, as a data: URL.
func mockIcon(colour string) string {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect x="14" y="8" width="36" height="16" rx="2" fill="#fff" stroke="#8a94a6" stroke-width="2"/>` +
		`<rect x="6" y="22" width="52" height="26" rx="6" fill="` + colour + `"/><rect x="14" y="38" width="36" height="18" rx="2" fill="#fff" stroke="#8a94a6" stroke-width="2"/>` +
		`<circle cx="50" cy="30" r="2.5" fill="#fff"/></svg>`
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}
