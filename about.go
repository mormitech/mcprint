package main

import (
	_ "embed"
	"runtime"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/appicon.png
var appIcon []byte

const (
	aboutLine    = "Scan pages of documents from network scanners (eSCL / AirScan) on macOS, Windows and Linux."
	aboutLicense = "Free and open source, MIT license."
	aboutURL     = "https://github.com/mormitech/mcprint"
)

func aboutText() string {
	return "Version " + version() + "\n\n" + aboutLine + "\n\n" + aboutLicense + "\n" + aboutURL
}

// macAbout is the content of the standard About panel in the application menu on macOS.
func macAbout() *mac.AboutInfo {
	return &mac.AboutInfo{Title: "mcprint", Message: aboutText(), Icon: appIcon}
}

// appMenu is the menu of the main window, the way each system does it. On macOS that is the application menu (with
// About, Hide, Quit), Edit and Window. On Windows and Linux there is a Help menu with About mcprint.
func appMenu(app *App) *menu.Menu {
	m := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		m.Append(menu.AppMenu())
		m.Append(menu.EditMenu())
		m.Append(menu.WindowMenu())
		return m
	}
	help := m.AddSubmenu("Help")
	help.AddText("About mcprint", nil, func(*menu.CallbackData) { app.showAbout() })
	return m
}

// showAbout opens the system's message box with the About text (Windows and Linux).
func (a *App) showAbout() {
	wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:    wruntime.InfoDialog,
		Title:   "About mcprint",
		Message: aboutText(),
	})
}
