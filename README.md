# mcprint

A platform-independent app for scanning pages of documents (macOS, Windows, Linux). Built like
[mcdiskfree](https://github.com/mormitech/mcdiskfree): Go + [Wails](https://wails.io), vanilla JS frontend.

**Status: v0.0.1, a skeleton.** It opens a window and shows its version. Scanning is not implemented yet.

## Develop

```
cd frontend && npm install && cd ..
wails dev          # live-reloading window
go test ./...
wails build        # the app ends up in build/bin
```

You need [Go](https://go.dev), Node and the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).
On Linux also `libgtk-3-dev libwebkit2gtk-4.1-dev`, and build with `-tags webkit2_41`.

## Versioning

The version lives in one place, `wails.json` (`info.productVersion`). It feeds the macOS Info.plist, the Windows file
properties, the window title and the UI. A release is cut by pushing a `v*` tag that matches it.

## License

[MIT](LICENSE)
