package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The scanned pages survive a restart: each page is a JPEG file and session.json holds their order, in the user's
// cache folder (~/Library/Caches/mormiprint/session, %LocalAppData%\mormiprint\session, ~/.cache/mormiprint/session).

type sessionPage struct {
	ID     int `json:"id"`
	Width  int `json:"width"`
	Height int `json:"height"`
	DPI    int `json:"dpi"`
}

type sessionFile struct {
	NextID int           `json:"nextId"`
	Pages  []sessionPage `json:"pages"`
}

func (a *App) sessionDir() string { return filepath.Join(a.cacheDir, "session") }

func pageFile(dir string, id int) string { return filepath.Join(dir, fmt.Sprintf("page-%d.jpg", id)) }

// writePageFile stores the image of a page; failures are ignored, the session is a convenience.
func (a *App) writePageFile(p *Page) {
	if os.MkdirAll(a.sessionDir(), 0o755) == nil {
		os.WriteFile(pageFile(a.sessionDir(), p.ID), p.Data, 0o644)
	}
}

func (a *App) removePageFile(id int) { os.Remove(pageFile(a.sessionDir(), id)) }

// saveSession writes the page order. Call it with a.mu held.
func (a *App) saveSession() {
	dir := a.sessionDir()
	if len(a.pages) == 0 {
		os.RemoveAll(dir)
		return
	}
	sf := sessionFile{NextID: a.nextID}
	for _, p := range a.pages {
		sf.Pages = append(sf.Pages, sessionPage{p.ID, p.Width, p.Height, p.DPI})
	}
	data, err := json.Marshal(sf)
	if err != nil || os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp := filepath.Join(dir, "session.json.tmp")
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, filepath.Join(dir, "session.json"))
	}
}

// loadSession restores the pages of the last run. Pages whose file is gone are skipped, stray files are removed.
func (a *App) loadSession() {
	dir := a.sessionDir()
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return
	}
	var sf sessionFile
	if json.Unmarshal(data, &sf) != nil {
		return
	}
	keep := map[string]bool{"session.json": true}
	var pages []*Page
	for _, sp := range sf.Pages {
		img, err := os.ReadFile(pageFile(dir, sp.ID))
		if err != nil {
			continue
		}
		pages = append(pages, &Page{ID: sp.ID, Data: img, Width: sp.Width, Height: sp.Height, DPI: sp.DPI})
		keep[filepath.Base(pageFile(dir, sp.ID))] = true
		if sp.ID >= sf.NextID {
			sf.NextID = sp.ID + 1
		}
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !keep[e.Name()] {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
	a.pages = pages
	if sf.NextID > a.nextID {
		a.nextID = sf.NextID
	}
}
