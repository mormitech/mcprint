package main

import (
	"context"
	"testing"

	"mcprint/internal/escl"
)

func TestMock(t *testing.T) {
	a := NewApp()
	if err := a.enableMock(); err != nil {
		t.Fatal(err)
	}
	defer func() { a.shutdown(nil) }()
	a.registry = a.mock.registry

	if got := len(a.Pages()); got != 4 {
		t.Fatalf("%d pages already scanned, want 4", got)
	}
	devs := a.registry.List()
	if len(devs) != 3 || !devs[0].Online || devs[2].Online {
		t.Fatalf("devices %+v", devs)
	}

	// the first scanner: a flatbed and a feeder that can scan both sides, toggles for everything
	ch, err := a.mock.caps[mockAlder].Choices("feeder", true)
	if err != nil || !ch.CanDuplex || len(ch.Sources) != 2 || len(ch.Colors) != 3 || len(ch.DPIs) != 5 {
		t.Fatalf("%+v %v", ch, err)
	}
	// the second one: a dpi range that fits the toggles, a feeder without both sides or black & white
	ch, _ = a.mock.caps[mockKestrel].Choices("platen", false)
	if len(ch.DPIs) != 6 || ch.CanDuplex {
		t.Fatalf("%+v", ch)
	}
	if ch, _ = a.mock.caps[mockKestrel].Choices("feeder", false); len(ch.Colors) != 2 {
		t.Fatalf("%+v", ch)
	}

	// an offline scanner cannot be read, and nothing can be added
	if _, err := a.capabilities(context.Background(), mockHeron); err == nil {
		t.Error("the offline scanner should not answer")
	}
	if _, err := a.AddDevice("192.168.1.99"); err == nil {
		t.Error("adding a scanner is off in demo mode")
	}

	// a scan from the feeder, both sides, brings four pages
	r, _ := a.mock.caps[mockAlder].Resolve(escl.Options{Source: "feeder", Duplex: true})
	n, err := a.mock.scan(context.Background(), r, func(b []byte) { a.addPage(b, 150) })
	if err != nil || n != 4 || len(a.Pages()) != 8 {
		t.Fatalf("n=%d err=%v pages=%d", n, err, len(a.Pages()))
	}
}
