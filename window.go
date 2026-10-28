package main

import (
	"context"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// fitSize limits a window of w x h to 90% of the screen, so the title bar stays reachable when the system's scaling is high
// (1366x768 at 125% is about 1093x614 in the logical pixels Wails uses). The screen size includes the taskbar or dock, hence
// the margin. A screen of unknown size leaves the window alone.
func fitSize(w, h, screenW, screenH int) (int, int) {
	if screenW <= 0 || screenH <= 0 {
		return w, h
	}
	return min(w, screenW*9/10), min(h, screenH*9/10)
}

// fitWindow shrinks the window to the screen it is on, and centres it if it had to. The window is created at its full size
// first, so a brief flash at that size is possible.
func fitWindow(ctx context.Context, w, h int) {
	screens, err := wruntime.ScreenGetAll(ctx)
	if err != nil {
		return
	}
	for _, s := range screens {
		if !s.IsCurrent {
			continue
		}
		fw, fh := fitSize(w, h, s.Size.Width, s.Size.Height)
		if fw != w || fh != h {
			wruntime.WindowSetSize(ctx, fw, fh)
			wruntime.WindowCenter(ctx)
		}
		return
	}
}
