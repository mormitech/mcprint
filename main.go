package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io"
	"os"

	"github.com/wailsapp/wails/v2"
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

// infoFlag reads --info <scanner id>: show the details window of that scanner instead of the app. The app starts
// itself this way when the info button is clicked, because Wails v2 has one window per process.
func infoFlag(args []string) string {
	var guid string
	fs := flag.NewFlagSet("mcprint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&guid, "info", "", "show the details window of this scanner")
	fs.Parse(args) // other arguments, such as the ones macOS adds when it launches an app, are ignored
	return guid
}

func main() {
	app := NewApp()
	title, width, height := "mcprint v"+version(), 1024, 768
	if guid := infoFlag(os.Args[1:]); guid != "" {
		app.infoGUID = guid
		title, width, height = "Scanner details", 600, 720
	} else {
		app.loadSession()
	}

	err := wails.Run(&options.App{
		Title:  title,
		Width:  width,
		Height: height,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.pageHandler(),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		// Without Mac options Wails leaves the green zoom (maximise) button disabled.
		Mac:        &mac.Options{}, // the window follows the system's light or dark appearance
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
