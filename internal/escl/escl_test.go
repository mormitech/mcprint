package escl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func loadCaps(t *testing.T) *Caps {
	data, err := os.ReadFile("testdata/capabilities.xml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCaps(data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseCaps(t *testing.T) {
	c := loadCaps(t)
	if c.Model != "Example MFC-1000 series" || c.UUID != "11111111-2222-3333-4444-555555555555" || c.FeederCapacity != 20 {
		t.Fatalf("%+v", c)
	}
	if c.Platen == nil || c.Simplex == nil || c.Duplex != nil {
		t.Fatalf("inputs: %+v", c)
	}
	if got := c.Platen.Resolutions.Values; len(got) != 4 || got[3] != 600 {
		t.Errorf("platen dpi %v", got)
	}
	if len(c.Simplex.ColorModes) != 2 || c.Platen.MaxHeight != 3507 {
		t.Errorf("simplex %+v", c.Simplex)
	}
}

func TestParseCapsRejectsOther(t *testing.T) {
	if _, err := ParseCaps([]byte("<html/>")); err == nil {
		t.Error("expected an error")
	}
}

func TestChoicesAndResolve(t *testing.T) {
	c := loadCaps(t)
	ch, err := c.Choices("", false)
	if err != nil || len(ch.Sources) != 2 || ch.CanDuplex || len(ch.DPIs) != 2 || ch.DefaultDP != 300 {
		t.Fatalf("%+v %v", ch, err)
	}
	if ch, _ = c.Choices("platen", false); len(ch.Colors) != 3 {
		t.Errorf("platen colours %v", ch.Colors)
	}
	r, err := c.Resolve(Options{})
	if err != nil || r.Source != "feeder" || r.Color != "color" || r.DPI != 300 || r.Width != 2480 || r.Height != 3507 {
		t.Fatalf("%+v %v", r, err)
	}
	r, _ = c.Resolve(Options{Source: "feeder", Paper: "legal"})
	if r.Width != 2550 || r.Height != 4200 {
		t.Errorf("legal %+v", r)
	}
	r, _ = c.Resolve(Options{Source: "platen", Paper: "max"})
	if r.Width != 2550 || r.Height != 3507 {
		t.Errorf("max %+v", r)
	}
	for _, bad := range []Options{
		{Source: "feeder", Duplex: true},
		{Source: "platen", Duplex: true},
		{DPI: 1234},
		{Source: "feeder", Color: "bw"},
		{Color: "sepia"},
		{Paper: "tabloid"},
	} {
		if _, err := c.Resolve(bad); err == nil {
			t.Errorf("%+v should fail", bad)
		}
	}
}

func TestSettingsXML(t *testing.T) {
	c := loadCaps(t)
	r, _ := c.Resolve(Options{Source: "feeder"})
	x := SettingsXML(r)
	for _, want := range []string{"<pwg:InputSource>Feeder", "<scan:Duplex>false", "<scan:XResolution>300", "RGB24", "<pwg:Width>2480"} {
		if !strings.Contains(x, want) {
			t.Errorf("missing %q in\n%s", want, x)
		}
	}
	r, _ = c.Resolve(Options{Source: "platen"})
	if strings.Contains(SettingsXML(r), "Duplex") {
		t.Error("the flatbed has no duplex element")
	}
}

func TestGUID(t *testing.T) {
	if g := GUID("urn:uuid:ABCDEF01-2222-3333-4444-555555555555", "h", "m"); g != "abcdef01-2222-3333-4444-555555555555" {
		t.Error(g)
	}
	a, b := GUID("", "printer.local", "X"), GUID("", "PRINTER.local", "X")
	if a != b || !uuidRe.MatchString(a) || a == GUID("", "other.local", "X") {
		t.Error(a, b)
	}
}

func TestIsLAN(t *testing.T) {
	for host, want := range map[string]bool{"192.168.1.5": true, "10.0.0.1:80": true, "127.0.0.1": true, "172.20.1.1": true,
		"8.8.8.8": false, "printer.local": true, "169.254.1.1": true, "[::1]:80": true} {
		if IsLAN(host) != want {
			t.Errorf("IsLAN(%q) != %v", host, want)
		}
	}
	if _, err := NewClient("http://8.8.8.8/eSCL"); err == nil {
		t.Error("public address must be refused")
	}
}

// fakeScanner serves two pages, answering 404 once between them like a Brother does.
func fakeScanner(t *testing.T, pages int) (*httptest.Server, *int32) {
	var next, deleted int32
	mux := http.NewServeMux()
	mux.HandleFunc("/eSCL/ScannerStatus", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<scan:ScannerStatus xmlns:scan="x" xmlns:pwg="y"><pwg:State>Idle</pwg:State><scan:AdfState>ScannerAdfLoaded</scan:AdfState></scan:ScannerStatus>`))
	})
	mux.HandleFunc("/eSCL/ScanJobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/xml" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Location", "/eSCL/ScanJobs/1")
		w.WriteHeader(201)
	})
	mux.HandleFunc("/eSCL/ScanJobs/1/NextDocument", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&next, 1)
		if n == 2 { // too soon after the first page
			w.WriteHeader(404)
			return
		}
		if int(n) > pages+1 {
			w.WriteHeader(404)
			return
		}
		w.Write([]byte("jpeg" + string(rune('0'+n))))
	})
	mux.HandleFunc("/eSCL/ScanJobs/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			atomic.AddInt32(&deleted, 1)
		}
	})
	return httptest.NewServer(mux), &deleted
}

func TestScanFeeder(t *testing.T) {
	srv, deleted := fakeScanner(t, 2)
	defer srv.Close()
	c, err := NewClient(srv.URL + "/eSCL")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	n, err := c.Scan(context.Background(), Resolved{Options: Options{Source: "feeder", Color: "color", DPI: 300}, Width: 100, Height: 100},
		func(b []byte) { got = append(got, string(b)) })
	if err != nil || n != 2 || len(got) != 2 || got[0] != "jpeg1" {
		t.Fatalf("n=%d err=%v got=%v", n, err, got)
	}
	if atomic.LoadInt32(deleted) != 1 {
		t.Error("job was not deleted")
	}
}

func TestScanCancel(t *testing.T) {
	srv, deleted := fakeScanner(t, 5)
	defer srv.Close()
	c, _ := NewClient(srv.URL + "/eSCL")
	ctx, cancel := context.WithCancel(context.Background())
	_, err := c.Scan(ctx, Resolved{Options: Options{Source: "feeder", Color: "color", DPI: 300}}, func([]byte) { cancel() })
	if err == nil {
		t.Error("expected the scan to be cancelled")
	}
	if atomic.LoadInt32(deleted) != 1 {
		t.Error("job was not deleted after cancel")
	}
}
