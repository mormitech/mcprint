package main

import "testing"

func TestSessionRoundTrip(t *testing.T) {
	a := &App{cacheDir: t.TempDir(), nextID: 1}
	for i, data := range []string{"one", "two", "three"} {
		p := &Page{ID: a.nextID, Data: []byte(data), Width: 10 + i, Height: 20, DPI: 300}
		a.nextID++
		a.pages = append(a.pages, p)
		a.writePageFile(p)
	}
	a.DeletePage(2)
	a.MovePage(3, -1)

	b := &App{cacheDir: a.cacheDir, nextID: 1}
	b.loadSession()
	if len(b.pages) != 2 || b.pages[0].ID != 3 || string(b.pages[0].Data) != "three" || b.pages[1].ID != 1 || b.pages[0].Width != 12 {
		t.Fatalf("restored %+v", b.pages)
	}
	if b.nextID != 4 {
		t.Errorf("nextID %d, ids must not be reused", b.nextID)
	}

	b.ClearPages()
	c := &App{cacheDir: a.cacheDir, nextID: 1}
	c.loadSession()
	if len(c.pages) != 0 {
		t.Error("cleared session came back")
	}
}
