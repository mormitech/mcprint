package main

import "testing"

func TestFitSize(t *testing.T) {
	for _, c := range []struct{ w, h, sw, sh, fw, fh int }{
		{1440, 800, 1920, 1080, 1440, 800}, // fits
		{1440, 800, 1093, 614, 983, 552},   // 1366x768 at 125%
		{1440, 800, 1280, 720, 1152, 648},  // 1280x720
		{600, 720, 1093, 614, 600, 552},    // only the height is too big
		{1440, 800, 0, 0, 1440, 800},       // screen unknown
	} {
		if fw, fh := fitSize(c.w, c.h, c.sw, c.sh); fw != c.fw || fh != c.fh {
			t.Errorf("fitSize(%d,%d,%d,%d) = %d,%d; want %d,%d", c.w, c.h, c.sw, c.sh, fw, fh, c.fw, c.fh)
		}
	}
}
