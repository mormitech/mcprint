# mcprint

A platform-independent app for scanning pages of documents (macOS, Windows, Linux). Built like
[mcdiskfree](https://github.com/mormitech/mcdiskfree): Go + [Wails](https://wails.io), vanilla JS frontend.

**Status: v0.1.0.** Finds eSCL (AirScan) scanners on the network, scans from the flatbed or the feeder, shows the pages as they
arrive and saves them as one PDF.

## What it does

- **Discovery.** Browses mDNS for `_uscan._tcp` and `_uscans._tcp` every few seconds. A scanner that offers both is kept once (HTTP wins).
  Scanners are remembered between runs; ones that miss the mDNS window are asked directly before they are shown as offline.
  A scanner on a network where mDNS does not get through can be added by address.
- **Selectable scanners.** Pick one in the list; its capabilities (sources, colour modes, resolutions, paper sizes, two-sided) decide what the form offers,
  and your choices are remembered per scanner.
- **Scan.** Pages appear one by one while the feeder works; Cancel stops the job on the scanner. Reorder or delete pages, click one to enlarge, **Save as PDF**.
- **Remembers the session.** Scanned pages are kept in the user's cache folder (`~/Library/Caches/mcprint/session`, `%LocalAppData%\mcprint\session`, `~/.cache/mcprint/session`) and come back when the app is reopened, until you clear them.
- **Safe.** Only local-network addresses are ever contacted. Nothing is uploaded; the only files written are the device cache, the last session's pages and your preferences.

Only network scanners (eSCL) for now; USB-only scanners are not supported. See [docs/scanner-protocol.md](docs/scanner-protocol.md).

## Develop

```
cd frontend && npm install && cd ..
wails dev          # live-reloading window
go test ./...
wails build        # the app ends up in build/bin
```

You need [Go](https://go.dev), Node and the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).
On Linux also `libgtk-3-dev libwebkit2gtk-4.1-dev`, and build with `-tags webkit2_41`.

## Layout

| Path | What |
|---|---|
| `app.go` | the methods the UI calls: devices, scan, pages, PDF export, preferences |
| `internal/escl` | eSCL: capabilities, options, scan client, device registry with mDNS discovery |
| `internal/pdf` | PDF writer (JPEGs embedded as they are) |
| `frontend/` | the UI |

## Versioning

The version lives in one place, `wails.json` (`info.productVersion`). It feeds the macOS Info.plist, the Windows file
properties, the window title and the UI. A release is cut by pushing a `v*` tag that matches it.

## License

[MIT](LICENSE)
