<h1 align="center">mcPrint</h1>

<p align="center">
  <b>Scan pages of documents from your network scanner.</b><br>
  A free, open-source scanning app for macOS, Windows and Linux.
</p>

<p align="center">
  <a href="https://github.com/mormitech/mcprint/actions/workflows/build.yml"><img alt="build" src="https://github.com/mormitech/mcprint/actions/workflows/build.yml/badge.svg"></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <img alt="platforms" src="https://img.shields.io/badge/platforms-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg">
  <img alt="status" src="https://img.shields.io/badge/status-early%20preview-orange.svg">
</p>

---

## What it does

mcPrint finds the scanners on your network, lets you pick one, scans from the flatbed or the document feeder, and collects the pages
into one PDF. No driver, no vendor software, no account.

- **Finds your scanners.** It looks for eSCL (AirScan) scanners with mDNS (Bonjour) and remembers them, so they are there the moment
  you start the app. A scanner that does not answer mDNS (some networks block it) can be added by its address.
- **Only what your scanner can do.** Source (flatbed or feeder), both sides, colour, resolution and paper size are offered as toggles,
  from what the scanner reports; your choices are remembered per scanner. The **i** button shows everything the scanner told us.
- **Pages show up as they are scanned.** Feed a stack and watch the pages arrive; **Cancel** stops the job on the scanner. When something
  goes wrong (feeder empty, scanner off), there is a **Reconnect** or **Try again** button.
- **Page view.** Click a page to see it large: scroll to zoom, drag to move, double-click to fit or zoom in, arrow keys to flip pages.
- **Rearrange.** Drag a page to a new place and see where it will land, or use the arrows; delete pages you do not want.
- **Save as PDF.** One PDF with all pages, in the order shown. The scanned JPEGs are embedded as they came from the scanner.
- **Picks up where you left off.** The pages of the last session are restored when you reopen the app.
- **Follows your system.** Light or dark mode, and the About window in the place each system puts it.

Only network scanners (eSCL / AirScan) are supported. USB-only scanners are not, yet. How the scanner talk works is in
[docs/scanner-protocol.md](docs/scanner-protocol.md).

## Install

mcPrint is an **early preview**. Download it from the [latest release](https://github.com/mormitech/mcprint/releases/latest):

| | |
|---|---|
| **macOS** (Apple silicon) | `mcprint-<version>-macos-arm64.zip`: unzip and drag `mcprint.app` to Applications. The app is not notarized, so the first time use **Open Anyway** in System Settings → Privacy & Security. |
| **Windows** (x64) | `mcprint-setup-<version>-windows-x64.exe`, no admin rights needed (`/VERYSILENT` installs without any window). SmartScreen shows **More info → Run anyway**. There is also a plain `.zip`. |
| **Linux** (x64) | `mcprint-<version>-linux-x64.tar.gz`: extract and run `mcprint`. You need GTK 3 and WebKitGTK 4.1 (`libgtk-3-0` and `libwebkit2gtk-4.1-0` on Debian and Ubuntu). |

Or with Homebrew, on macOS and Linux (from the [mormitech tap](https://github.com/mormitech/homebrew-tap)):

```
brew install --cask mormitech/tap/mcprint   # macOS, Apple silicon
brew install mormitech/tap/mcprint          # Linux x64
```

### Platform status

| | macOS | Windows | Linux |
|---|---|---|---|
| Builds and tests in CI | ✅ | ✅ (installer installed, started and uninstalled on a runner) | ✅ |
| Used and tried by hand | ✅ (Apple silicon, one Brother scanner) | ⏳ not yet | ⏳ not yet |

Releases are not code-signed. Found a scanner that does not work? [Open an issue](https://github.com/mormitech/mcprint/issues) and say which model it is.

## Privacy

mcPrint talks to scanners on your local network and to nothing else: it refuses any address that is not a local one, and it
sends nothing anywhere (no telemetry, nothing uploaded). It writes three things, which you can delete at any time:

| What | Where (macOS · Windows · Linux) |
|---|---|
| Known scanners, last session's pages | `~/Library/Caches/mcprint` · `%LocalAppData%\mcprint` · `~/.cache/mcprint` |
| Your settings | `~/Library/Application Support/mcprint` · `%AppData%\mcprint` · `~/.config/mcprint` |

## Develop

```
cd frontend && npm install && cd ..
wails dev          # live-reloading window
go vet ./... && go test ./...
wails build        # the app ends up in build/bin
```

You need [Go](https://go.dev), Node and the [Wails](https://wails.io) CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).
On Linux also `libgtk-3-dev libwebkit2gtk-4.1-dev`, and build with `-tags webkit2_41`.

| Path | What |
|---|---|
| `main.go`, `about.go` | the window, the menus per system, the `--about` and `--info` windows |
| `app.go`, `session.go` | the methods the UI calls: scanners, scan, pages, PDF, settings; the saved session |
| `internal/escl` | eSCL: capabilities, options, the scan client, mDNS discovery and the scanner list |
| `internal/pdf` | PDF writer (JPEGs embedded as they are) |
| `frontend/` | the UI (vanilla JS, built with Vite) |
| `packaging/windows` | the Inno Setup installer script |
| `mock.go`, `mock/` | demo mode: made-up scanners and pages (`scripts/make-mock.mjs` draws the pages) |
| `scripts/` | generators for the Homebrew cask and formula (`make-cask.mjs`, `make-formula.mjs`) |
| `.github/workflows/build.yml` | builds, tests and packages on macOS, Windows and Linux; publishes the release on a tag |

The window has one process per window (Wails v2): the About window and a scanner's details window are the same program started
with `--about` or `--info <scanner id>`.

**Demo mode** shows made-up scanners and already scanned pages instead of yours, handy for screenshots. Nothing real is searched,
scanned or saved:

```
mcprint --mock        # the macOS binary is mcprint.app/Contents/MacOS/mcprint; --demo works too
```

The pages are drawn by `scripts/make-mock.mjs` (committed in `mock/pages`). `--theme dark` (or `light`) forces the look instead of the
system's, and the screenshots in `docs/screenshots` were taken this way, at a window size of 1280 × 832.

**Versioning.** The version lives in one place, `wails.json` (`info.productVersion`). It feeds the macOS Info.plist, the Windows file
properties, the window title and the About window. A release is cut by pushing a `v*` tag that matches it; CI checks that, builds the
packages and attaches them to a GitHub release.

## Roadmap

Ideas, not promises:

1. **Try it on Windows and Linux** by hand, and on more scanners.
2. **Homebrew** (macOS and Linux), winget, and signing.
3. **USB and older scanners**: SANE on Linux, WIA on Windows, ImageCaptureCore on macOS.
4. **Pages**: rotate, crop, and a PDF you can search (OCR).

## License

[MIT](LICENSE): use it, change it, ship it, forever.
