package main

import "testing"

func TestVersion(t *testing.T) {
	if v := version(); v == "" || v == "dev" {
		t.Fatalf("version() = %q, want the productVersion from wails.json", v)
	}
}
