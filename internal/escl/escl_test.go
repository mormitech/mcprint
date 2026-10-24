package escl

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
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

func TestUnreachable(t *testing.T) {
	c, _ := NewClient("http://127.0.0.1:1/eSCL") // nothing listens there
	_, err := c.Caps(context.Background())
	if err == nil || !Unreachable(err) {
		t.Fatalf("a refused connection should count as unreachable, got %v", err)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c, _ = NewClient(srv.URL)
	if _, err := c.Caps(context.Background()); err == nil || Unreachable(err) {
		t.Fatalf("an HTTP answer is not unreachable, got %v", err)
	}
}

func TestToBlackAndWhite(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			v := uint8(215) // light grey paper
			if x < 32 {
				v = 70 // dark grey ink
			}
			src.SetGray(x, y, color.Gray{Y: v})
		}
	}
	var in bytes.Buffer
	if err := jpeg.Encode(&in, src, nil); err != nil {
		t.Fatal(err)
	}
	out, err := ToBlackAndWhite(in.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if l, r := color.GrayModel.Convert(img.At(8, 32)).(color.Gray).Y, color.GrayModel.Convert(img.At(56, 32)).(color.Gray).Y; l > 40 || r < 215 {
		t.Fatalf("ink should be black and paper white, got %d and %d", l, r)
	}
}

// A Brother-like scanner: it refuses black & white (a 1-bit image cannot be a JPEG) but scans grey.
func TestScanBlackAndWhiteFallsBackToGrey(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 32, 32))
	for i := range src.Pix {
		src.Pix[i] = 200
	}
	var page bytes.Buffer
	jpeg.Encode(&page, src, nil)
	var modes []string
	mux := http.NewServeMux()
	mux.HandleFunc("/eSCL/ScannerStatus", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<scan:ScannerStatus xmlns:scan="x" xmlns:pwg="y"><pwg:State>Idle</pwg:State></scan:ScannerStatus>`))
	})
	mux.HandleFunc("/eSCL/ScanJobs", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		buf.ReadFrom(r.Body)
		mode := "Grayscale8"
		if strings.Contains(buf.String(), "BlackAndWhite1") {
			mode = "BlackAndWhite1"
		}
		modes = append(modes, mode)
		if mode == "BlackAndWhite1" {
			w.WriteHeader(409)
			return
		}
		w.Header().Set("Location", "/eSCL/ScanJobs/1")
		w.WriteHeader(201)
	})
	mux.HandleFunc("/eSCL/ScanJobs/1/NextDocument", func(w http.ResponseWriter, r *http.Request) { w.Write(page.Bytes()) })
	mux.HandleFunc("/eSCL/ScanJobs/1", func(w http.ResponseWriter, r *http.Request) {})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, _ := NewClient(srv.URL + "/eSCL")
	var got [][]byte
	n, err := c.Scan(context.Background(), Resolved{Options: Options{Source: "platen", Color: "bw", DPI: 300}, Width: 100, Height: 100},
		func(b []byte) { got = append(got, b) })
	if err != nil || n != 1 || len(got) != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(modes) != 2 || modes[0] != "BlackAndWhite1" || modes[1] != "Grayscale8" {
		t.Fatalf("expected black & white first, then grey, got %v", modes)
	}
	img, err := jpeg.Decode(bytes.NewReader(got[0]))
	if err != nil {
		t.Fatal(err)
	}
	if v := color.GrayModel.Convert(img.At(5, 5)).(color.Gray).Y; v != 255 && v != 0 {
		t.Errorf("page is not black and white: %d", v)
	}
}

func TestParseStatusJobs(t *testing.T) {
	st, err := ParseStatus([]byte(`<scan:ScannerStatus xmlns:scan="x" xmlns:pwg="y"><pwg:State>Idle</pwg:State><scan:Jobs>
<scan:JobInfo><pwg:JobUri>/eSCL/ScanJobs/a</pwg:JobUri><pwg:ImagesCompleted>1</pwg:ImagesCompleted><pwg:ImagesToTransfer>0</pwg:ImagesToTransfer><pwg:JobState>Canceled</pwg:JobState><pwg:JobStateReasons><pwg:JobStateReason>JobCanceledAtDevice</pwg:JobStateReason></pwg:JobStateReasons></scan:JobInfo>
</scan:Jobs></scan:ScannerStatus>`))
	if err != nil || len(st.Jobs) != 1 || st.Jobs[0].State != "Canceled" || st.Jobs[0].Done != 1 || st.Jobs[0].Reasons[0] != "JobCanceledAtDevice" {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestClearJobs(t *testing.T) {
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("/eSCL/ScannerStatus", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<scan:ScannerStatus xmlns:scan="x" xmlns:pwg="y"><pwg:State>Idle</pwg:State><scan:Jobs>
<scan:JobInfo><pwg:JobUri>/eSCL/ScanJobs/a</pwg:JobUri><pwg:JobState>Canceled</pwg:JobState></scan:JobInfo>
<scan:JobInfo><pwg:JobUri>http://8.8.8.8/eSCL/ScanJobs/evil</pwg:JobUri><pwg:JobState>Pending</pwg:JobState></scan:JobInfo>
<scan:JobInfo><pwg:JobUri>/eSCL/ScanJobs/b</pwg:JobUri><pwg:JobState>Processing</pwg:JobState></scan:JobInfo>
</scan:Jobs></scan:ScannerStatus>`))
	})
	mux.HandleFunc("/eSCL/ScanJobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, _ := NewClient(srv.URL + "/eSCL")
	n, err := c.ClearJobs(context.Background())
	if err != nil || n != 2 || len(deleted) != 2 || deleted[0] != "/eSCL/ScanJobs/a" || deleted[1] != "/eSCL/ScanJobs/b" {
		t.Fatalf("n=%d err=%v deleted=%v", n, err, deleted)
	}
}
