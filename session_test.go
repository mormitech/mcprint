package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

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

func TestMovePageTo(t *testing.T) {
	a := &App{cacheDir: t.TempDir(), nextID: 1}
	for i := 0; i < 4; i++ {
		a.pages = append(a.pages, &Page{ID: a.nextID, Data: []byte{byte(i)}})
		a.nextID++
	}
	ids := func() (out []int) {
		for _, p := range a.Pages() {
			out = append(out, p.ID)
		}
		return
	}
	for _, c := range []struct {
		id, to int
		want   [4]int
	}{
		{4, 0, [4]int{4, 1, 2, 3}},
		{4, 3, [4]int{1, 2, 3, 4}},
		{1, 2, [4]int{2, 3, 1, 4}},
		{2, 99, [4]int{3, 1, 4, 2}},
		{9, 0, [4]int{3, 1, 4, 2}},
	} {
		a.MovePageTo(c.id, c.to)
		got := ids()
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Fatalf("move %d to %d: got %v, want %v", c.id, c.to, got, c.want)
			}
		}
	}
}

func TestAddPageAt(t *testing.T) {
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), nil)
	a := &App{cacheDir: t.TempDir(), nextID: 1}
	order := func() (out []int) {
		for _, p := range a.Pages() {
			out = append(out, p.ID)
		}
		return
	}
	a.addPage(buf.Bytes(), 300)       // 1
	a.addPage(buf.Bytes(), 300)       // 2: 1 2
	a.addPageAt(buf.Bytes(), 300, 1)  // 3 between them: 1 3 2
	a.addPageAt(buf.Bytes(), 300, 0)  // 4 first: 4 1 3 2
	a.addPageAt(buf.Bytes(), 300, 99) // 5 past the end goes last: 4 1 3 2 5
	a.addPageAt(buf.Bytes(), 300, -1) // 6 last
	want := []int{4, 1, 3, 2, 5, 6}
	got := order()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
