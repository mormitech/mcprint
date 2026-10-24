package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mcprint/internal/escl"
	"mcprint/internal/pdf"
)

// Page is one scanned page, kept in memory until the user clears it.
type Page struct {
	ID     int
	Data   []byte
	Width  int
	Height int
	DPI    int
}

// PageView is what the UI gets to know about a page; the image itself is served at /page/<id>.jpg.
type PageView struct {
	ID     int `json:"id"`
	Width  int `json:"width"`
	Height int `json:"height"`
	DPI    int `json:"dpi"`
	Bytes  int `json:"bytes"`
}

// ScanRequest is a scan started from the UI.
type ScanRequest struct {
	GUID string `json:"guid"`
	escl.Options
	InsertAt int `json:"insertAt"` // where the first scanned page goes (0 is before the first page); negative: at the end
}

// ScanEvent is sent to the UI as the "scan" event.
type ScanEvent struct {
	State   string `json:"state"` // scanning, done, cancelled or error
	Message string `json:"message"`
	Count   int    `json:"count"` // pages received in this scan
}

type capsEntry struct {
	caps *escl.Caps
	at   time.Time
}

// App holds the methods the UI calls.
type App struct {
	ctx      context.Context
	registry *escl.Registry
	cacheDir string
	confDir  string

	mu       sync.Mutex
	caps     map[string]capsEntry
	icons    map[string]string // the pictures scanners offer of themselves, by scanner
	pages    []*Page
	nextID   int
	theme    string     // --theme: light or dark instead of the system's setting
	mock     *mockState // set in demo mode (--mock)
	mode     string     // "" for the app, "info" for a scanner's details window, "about" for the About window
	infoGUID string     // the scanner of the details window
	infoWins map[string]*exec.Cmd
	scanning bool
}

func NewApp() *App {
	cache, _ := os.UserCacheDir()
	conf, _ := os.UserConfigDir()
	a := &App{
		cacheDir: filepath.Join(cache, "mcprint"),
		confDir:  filepath.Join(conf, "mcprint"),
		caps:     map[string]capsEntry{},
		icons:    map[string]string{},
		nextID:   1,
	}
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.mock != nil { // demo mode: the made-up scanners, no searching
		a.registry = a.mock.registry
		a.registry.SetOnChange(func() { wruntime.EventsEmit(a.ctx, "devices", a.registry.List()) })
		a.titleWindow(ctx)
		return
	}
	if a.mode != "" { // a small window of its own: no searching and no session
		if a.mode == "info" || a.mode == "extra" { // read the saved device list
			a.registry = escl.NewRegistry(filepath.Join(a.cacheDir, "devices.json"), nil)
			a.titleWindow(ctx)
		}
		return
	}
	a.registry = escl.NewRegistry(filepath.Join(a.cacheDir, "devices.json"), func() {
		wruntime.EventsEmit(a.ctx, "devices", a.registry.List())
	})
	a.registry.OnSweep = func(searching bool) { wruntime.EventsEmit(a.ctx, "searching", searching) }
	go a.registry.Run(ctx)
}

// titleWindow names a scanner's window after the scanner.
func (a *App) titleWindow(ctx context.Context) {
	d, ok := a.registry.Get(a.infoGUID)
	if !ok {
		return
	}
	switch a.mode {
	case "info":
		wruntime.WindowSetTitle(ctx, d.Name+" – details")
	case "extra":
		wruntime.WindowSetTitle(ctx, d.Name+" – additional info")
	}
}

// Version is the app version from wails.json.
func (a *App) Version() string { return version() }

// ---- Devices ------------------------------------------------------------------------------------------------

// Devices lists the known scanners, online ones first. They are also pushed as the "devices" event.
func (a *App) Devices() []escl.Device { return a.registry.List() }

// Search looks for scanners right now instead of waiting for the next round.
func (a *App) Search() { a.registry.Search() }

// AddDevice adds a scanner by address, for networks where mDNS does not get through.
func (a *App) AddDevice(address string) (escl.Device, error) {
	if a.mock != nil {
		return escl.Device{}, errors.New("demo mode: scanners cannot be added")
	}
	return a.registry.Add(a.ctx, address)
}

// RemoveDevice forgets a scanner.
func (a *App) RemoveDevice(guid string) {
	a.registry.Remove(guid)
	a.mu.Lock()
	delete(a.caps, guid)
	a.mu.Unlock()
}

func (a *App) client(guid string) (*escl.Client, error) {
	d, ok := a.registry.Get(guid)
	if !ok {
		return nil, errors.New("unknown scanner")
	}
	return escl.NewClient(d.BaseURL())
}

// capabilities are cached for five minutes.
func (a *App) capabilities(ctx context.Context, guid string) (*escl.Caps, error) {
	a.mu.Lock()
	e, ok := a.caps[guid]
	a.mu.Unlock()
	if ok && time.Since(e.at) < 5*time.Minute {
		return e.caps, nil
	}
	if a.mock != nil {
		if d, known := a.registry.Get(guid); known && d.Online {
			return a.mock.caps[guid], nil
		}
		return nil, errors.New("could not reach the scanner: is it switched on and on the same network?")
	}
	c, err := a.client(guid)
	if err != nil {
		return nil, err
	}
	caps, err := c.Caps(ctx)
	if err != nil {
		if escl.Unreachable(err) {
			return nil, errors.New("could not reach the scanner: is it switched on and on the same network?")
		}
		return nil, fmt.Errorf("could not read the scanner's capabilities: %w", err)
	}
	a.mu.Lock()
	a.caps[guid] = capsEntry{caps, time.Now()}
	a.mu.Unlock()
	return caps, nil
}

// Capabilities is what the scanner says it can do.
func (a *App) Capabilities(guid string) (*escl.Caps, error) { return a.capabilities(a.ctx, guid) }

// Choices lists what can be picked for a source of a scanner.
func (a *App) Choices(guid, source string, duplex bool) (escl.Choices, error) {
	caps, err := a.capabilities(a.ctx, guid)
	if err != nil {
		return escl.Choices{}, err
	}
	return caps.Choices(source, duplex)
}

// Status is the live state of the scanner (idle, processing, feeder loaded or empty).
func (a *App) Status(guid string) (escl.Status, error) {
	if a.mock != nil {
		if d, ok := a.registry.Get(guid); ok && d.Online {
			return escl.Status{State: "Idle", AdfState: "ScannerAdfLoaded"}, nil
		}
		return escl.Status{}, errors.New("the scanner is not reachable")
	}
	c, err := a.client(guid)
	if err != nil {
		return escl.Status{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	return c.Status(ctx)
}

// Icon is the picture the scanner offers of itself as a data: URL, or "" if it has none. A picture found is kept for the run.
func (a *App) Icon(guid string) string {
	a.mu.Lock()
	icon, ok := a.icons[guid]
	a.mu.Unlock()
	if ok {
		return icon
	}
	if a.mock != nil {
		return a.mock.icons[guid]
	}
	d, known := a.registry.Get(guid)
	if !known || !d.Online {
		return ""
	}
	ctx, cancel := context.WithTimeout(a.ctx, 12*time.Second)
	defer cancel()
	icon = escl.Icon(ctx, d)
	if icon != "" { // a failure is not remembered: the scanner may answer next time
		a.mu.Lock()
		a.icons[guid] = icon
		a.mu.Unlock()
	}
	return icon
}

// Extra is everything else the scanner offers: what it announced, links, a picture, jobs and usage counters. A scanner
// that has none of it gives an empty answer.
func (a *App) Extra(guid string) (escl.Extra, error) {
	d, ok := a.registry.Get(guid)
	if !ok {
		return escl.Extra{}, errors.New("unknown scanner")
	}
	if a.mock != nil {
		return escl.Extra{Sections: []escl.Section{}}, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return escl.Gather(ctx, d), nil
}

// ---- Scanning -----------------------------------------------------------------------------------------------

func (a *App) emitScan(ev ScanEvent) { wruntime.EventsEmit(a.ctx, "scan", ev) }

func (a *App) emitPages() { wruntime.EventsEmit(a.ctx, "pages", a.Pages()) }

// Scan starts a job and returns at once; the pages arrive one by one as the "pages" event, progress as "scan".
func (a *App) Scan(req ScanRequest) error {
	ctx0, cancelCaps := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancelCaps()
	caps, err := a.capabilities(ctx0, req.GUID)
	if err != nil {
		return err
	}
	resolved, err := caps.Resolve(req.Options)
	if err != nil {
		return err
	}
	var c *escl.Client
	if a.mock == nil {
		if c, err = a.client(req.GUID); err != nil {
			return err
		}
	}

	a.mu.Lock()
	if a.scanning {
		a.mu.Unlock()
		return errors.New("a scan is already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.scanning = true
	a.mu.Unlock()

	go func() {
		count := 0
		at := req.InsertAt
		a.emitScan(ScanEvent{State: "scanning", Message: "Scanning…"})
		onPage := func(data []byte) {
			if a.addPageAt(data, resolved.DPI, at) {
				if at >= 0 {
					at++ // the next page of a stack goes behind this one
				}
				count++
				a.emitPages()
				a.emitScan(ScanEvent{State: "scanning", Message: fmt.Sprintf("Scanning… %d page(s)", count), Count: count})
			}
		}
		var err error
		if a.mock != nil {
			_, err = a.mock.scan(ctx, resolved, onPage)
		} else {
			_, err = c.Scan(ctx, resolved, onPage)
		}
		a.mu.Lock()
		a.scanning = false
		a.mu.Unlock()
		cancel()
		switch {
		case errors.Is(err, context.Canceled):
			a.emitScan(ScanEvent{State: "cancelled", Message: "Scan cancelled", Count: count})
		case err != nil:
			msg := err.Error()
			a.emitScan(ScanEvent{State: "error", Message: strings.ToUpper(msg[:1]) + msg[1:], Count: count})
		default:
			a.emitScan(ScanEvent{State: "done", Message: fmt.Sprintf("Scanned %d page(s)", count), Count: count})
		}
	}()
	return nil
}

// addPage keeps a scanned page (and saves it in the session). It reports false for data that is not an image.
func (a *App) addPage(data []byte, dpi int) bool { return a.addPageAt(data, dpi, -1) }

// addPageAt is addPage with a position: 0 puts the page first, a negative position or one past the end puts it last.
func (a *App) addPageAt(data []byte, dpi int, at int) bool {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p := &Page{ID: a.nextID, Data: data, Width: cfg.Width, Height: cfg.Height, DPI: dpi}
	a.nextID++
	if at < 0 || at >= len(a.pages) {
		a.pages = append(a.pages, p)
	} else {
		a.pages = append(a.pages[:at], append([]*Page{p}, a.pages[at:]...)...)
	}
	a.writePageFile(p)
	a.saveSession()
	return true
}

// Scanning reports whether a scan is running, so the window can tell when it has lost track of one.
func (a *App) Scanning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.scanning
}

// ---- Pages --------------------------------------------------------------------------------------------------

// Pages lists the scanned pages in order.
func (a *App) Pages() []PageView {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]PageView, 0, len(a.pages))
	for _, p := range a.pages {
		out = append(out, PageView{ID: p.ID, Width: p.Width, Height: p.Height, DPI: p.DPI, Bytes: len(p.Data)})
	}
	return out
}

// DeletePage removes one page.
func (a *App) DeletePage(id int) []PageView {
	a.mu.Lock()
	for i, p := range a.pages {
		if p.ID == id {
			a.pages = append(a.pages[:i:i], a.pages[i+1:]...)
			a.removePageFile(id)
			a.saveSession()
			break
		}
	}
	a.mu.Unlock()
	return a.Pages()
}

// MovePage moves a page by delta positions (-1 earlier, +1 later).
func (a *App) MovePage(id, delta int) []PageView {
	a.mu.Lock()
	for i, p := range a.pages {
		if p.ID == id {
			j := i + delta
			if j >= 0 && j < len(a.pages) {
				a.pages[i], a.pages[j] = a.pages[j], a.pages[i]
				a.saveSession()
			}
			break
		}
	}
	a.mu.Unlock()
	return a.Pages()
}

// MovePageTo moves a page to a position (0 is the first page), as dropped by the user.
func (a *App) MovePageTo(id, index int) []PageView {
	a.mu.Lock()
	from := -1
	for i, p := range a.pages {
		if p.ID == id {
			from = i
			break
		}
	}
	if from >= 0 {
		index = max(0, min(index, len(a.pages)-1))
		p := a.pages[from]
		rest := append(append([]*Page{}, a.pages[:from]...), a.pages[from+1:]...)
		a.pages = append(rest[:index], append([]*Page{p}, rest[index:]...)...)
		a.saveSession()
	}
	a.mu.Unlock()
	return a.Pages()
}

// ClearPages drops all pages.
func (a *App) ClearPages() []PageView {
	a.mu.Lock()
	a.pages = nil
	a.saveSession() // removes the session folder
	a.mu.Unlock()
	return a.Pages()
}

// pageHandler serves /page/<id>.jpg to the UI.
func (a *App) pageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/page/"), ".jpg")
		id, err := strconv.Atoi(name)
		if err != nil || !strings.HasPrefix(r.URL.Path, "/page/") {
			http.NotFound(w, r)
			return
		}
		a.mu.Lock()
		var data []byte
		for _, p := range a.pages {
			if p.ID == id {
				data = p.Data
			}
		}
		a.mu.Unlock()
		if data == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
	})
}

// ExportPDF asks where to save and writes all pages as one PDF. It returns the path, or "" if the user cancelled.
func (a *App) ExportPDF() (string, error) {
	a.mu.Lock()
	pp := make([]pdf.Page, len(a.pages))
	for i, p := range a.pages {
		pp[i] = pdf.Page{JPEG: p.Data, DPI: p.DPI}
	}
	a.mu.Unlock()
	data, err := pdf.Build(pp)
	if err != nil {
		return "", err
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           "Save scan as PDF",
		DefaultFilename: "scan-" + time.Now().Format("20060102-150405") + ".pdf",
		Filters:         []wruntime.FileFilter{{DisplayName: "PDF (*.pdf)", Pattern: "*.pdf"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pdf") {
		path += ".pdf"
	}
	return path, os.WriteFile(path, data, 0o644)
}

// ---- Preferences --------------------------------------------------------------------------------------------

// LoadPrefs returns the saved UI preferences (last scanner, settings per scanner) as JSON.
func (a *App) LoadPrefs() string {
	data, err := os.ReadFile(filepath.Join(a.confDir, "prefs.json"))
	if err != nil {
		return "{}"
	}
	return string(data)
}

// SavePrefs stores the UI preferences.
func (a *App) SavePrefs(js string) error {
	if err := os.MkdirAll(a.confDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.confDir, "prefs.json"), []byte(js), 0o644)
}

// ---- Details window -----------------------------------------------------------------------------------------

// Mode tells the UI what to show: "" for the app, "about", or "info:<scanner id>" for the details of a scanner, "extra:<scanner id>" for its additional info.
func (a *App) Mode() string {
	if a.mode == "info" || a.mode == "extra" {
		return a.mode + ":" + a.infoGUID
	}
	return a.mode
}

// openWindow opens a window of its own: a second copy of this program, started with the given arguments, because
// Wails v2 has one window per process. A window that is already open (same key) is not opened twice.
func (a *App) openWindow(key string, args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.infoWins == nil {
		a.infoWins = map[string]*exec.Cmd{}
	}
	if c := a.infoWins[key]; c != nil && c.ProcessState == nil {
		return nil
	}
	if a.mock != nil {
		args = append([]string{"--mock"}, args...)
	}
	if a.theme != "" {
		args = append([]string{"--theme", a.theme}, args...)
	}
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	a.infoWins[key] = cmd
	go func() {
		cmd.Wait()
		a.mu.Lock()
		delete(a.infoWins, key)
		a.mu.Unlock()
	}()
	return nil
}

// ShowInfo opens the details of a scanner in a window of its own.
func (a *App) ShowInfo(guid string) error { return a.openWindow("info:"+guid, "--info", guid) }

// ShowExtra opens the additional info of a scanner in a window of its own.
func (a *App) ShowExtra(guid string) error { return a.openWindow("extra:"+guid, "--extra", guid) }

// showAbout opens the About window.
func (a *App) showAbout() { a.openWindow("about", "--about") }

// shutdown closes the details windows together with the app.
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mock != nil {
		os.RemoveAll(a.cacheDir) // the demo's temporary folder
	}
	for _, c := range a.infoWins {
		if c.Process != nil {
			c.Process.Kill()
		}
	}
}
