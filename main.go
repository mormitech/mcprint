package main

import (
	"embed"
	"encoding/json"
	"flag"
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

// windowFlags reads --info <scanner id> (show the details window of that scanner) and --about (show the About window)
// instead of the app. The app starts itself this way, because Wails v2 has one window per process.
func windowFlags(args []string) (info string, about bool) {
	fs := flag.NewFlagSet("mcprint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&info, "info", "", "show the details window of this scanner")
	fs.BoolVar(&about, "about", false, "show the About window")
	fs.Parse(args) // other arguments, such as the ones macOS adds when it launches an app, are ignored
	return
}

func main() {
	app := NewApp()
	title, width, height := "mcprint v"+version(), 1440, 800
	info, about := windowFlags(os.Args[1:])
	switch {
	case about:
		app.mode, title, width, height = "about", "About mcprint", 440, 520
	case info != "":
		app.mode, app.infoGUID, title, width, height = "info", info, "Scanner details", 600, 720
	default:
		app.loadSession()
	}

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
		Mac:        &mac.Options{}, // without Mac options the zoom button is disabled; the window follows the system's appearance
		Windows:    &windows.Options{Theme: windows.SystemDefault},
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
