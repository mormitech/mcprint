package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// wails.json is the single source of the version: it ends up in the macOS Info.plist and the Windows file properties,
// and is read from here for the window title and the UI.
//
//go:embed wails.json
var projectJSON []byte

func version() string {
	var p struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if json.Unmarshal(projectJSON, &p) != nil || p.Info.ProductVersion == "" {
		return "dev"
	}
	return p.Info.ProductVersion
}

// windowFlags reads --info <scanner id> (show the details window of that scanner), --extra <scanner id> (its additional info
// window) and --about (show the About window)
// instead of the app. The app starts itself this way, because Wails v2 has one window per process.
func windowFlags(args []string) (info, extra string, about, mock bool, theme string) {
	fs := flag.NewFlagSet("mcprint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&info, "info", "", "show the details window of this scanner")
	fs.StringVar(&extra, "extra", "", "show the additional info window of this scanner")
	fs.BoolVar(&about, "about", false, "show the About window")
	fs.BoolVar(&mock, "mock", false, "demo mode: made-up scanners and pages, for screenshots")
	fs.BoolVar(&mock, "demo", false, "same as --mock")
	fs.StringVar(&theme, "theme", "", "light or dark, instead of the system's setting (for screenshots)")
	fs.Parse(args) // other arguments, such as the ones macOS adds when it launches an app, are ignored
	return
}

func main() {
	app := NewApp()
	title, width, height := "mcPrint v"+version(), 1440, 800
	info, extra, about, mock, theme := windowFlags(os.Args[1:])
	switch {
	case about:
		app.mode, title, width, height = "about", "About mcPrint", 440, 520
	case info != "":
		app.mode, app.infoGUID, title, width, height = "info", info, "Scanner details", 600, 720
	case extra != "":
		app.mode, app.infoGUID, title, width, height = "extra", extra, "Additional info", 820, 860
	default:
		if !mock { // demo mode must not read the real session
			app.loadSession()
		}
	}

	if mock {
		if err := app.enableMock(); err != nil {
			fmt.Fprintln(os.Stderr, "mcprint:", err)
			os.Exit(2)
		}
	}

	app.theme = theme

	var appMenuBar *menu.Menu
	if app.mode == "" { // the small windows have no menu of their own
		appMenuBar = appMenu(app)
	}

	err := wails.Run(&options.App{
		Menu:          appMenuBar,
		Title:         title,
		Width:         width,
		Height:        height,
		DisableResize: about,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.pageHandler(),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		// Without Mac options Wails leaves the green zoom (maximise) button disabled.
		Mac:        &mac.Options{Appearance: macAppearance(theme)}, // without Mac options the zoom button is disabled
		Windows:    &windows.Options{Theme: windowsTheme(theme)},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// The window follows the system's light or dark appearance, unless --theme says otherwise (used for screenshots).
func macAppearance(theme string) mac.AppearanceType {
	switch theme {
	case "dark":
		return mac.NSAppearanceNameDarkAqua
	case "light":
		return mac.NSAppearanceNameAqua
	}
	return mac.DefaultAppearance
}

func windowsTheme(theme string) windows.Theme {
	switch theme {
	case "dark":
		return windows.Dark
	case "light":
		return windows.Light
	}
	return windows.SystemDefault
}
