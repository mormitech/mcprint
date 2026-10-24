# Scanner approach (from mctwain)

mctwain talks to network scanners with **eSCL (AirScan)** only; no TWAIN/WIA/SANE/ICA. mcprint will start the same way,
in Go (no CGO): mDNS browse + `net/http` + `encoding/xml`.

- **Discovery:** mDNS browse for `_uscan._tcp` (HTTP) and `_uscans._tcp` (HTTPS). Keep one entry per scanner, HTTP preferred.
  Device id = TXT `UUID`, else a stable hash of host+model. Root path = TXT `rs` (default `/eSCL`). Drop a device after 2 missed sweeps.
  Only LAN addresses (RFC1918, 127.x, *.local) are allowed.
- **Capabilities:** `GET {base}/ScannerCapabilities` (XML): colour modes, DPI (list or range), max size for platen / ADF simplex / ADF duplex.
  `GET {base}/ScannerStatus` for State/AdfState before each job.
- **Scan:** `POST {base}/ScanJobs` (ScanSettings XML, `Content-Type: application/xml`, element order as in sane-airscan) -> 201 + `Location`;
  retry on 503. Loop `GET {job}/NextDocument` (JPEG per page; 404/409 = done). `DELETE {job}` at the end.
- **Quirks:** Brother ignores `text/xml`; some ADF models need explicit `<scan:Duplex>false</scan:Duplex>`; ~1.5 s pause between feeder pages;
  HTTPS scanners use self-signed certificates (skip verification, LAN only).
- **Not covered:** USB-only/legacy scanners would need SANE (Linux), WIA (Windows), ImageCaptureCore (macOS).

# The existing web tool (mormitech-com, `src/mctools/mcprint/`)

React + pdf-lib page that talks to the mctwain helper at `127.0.0.1:7788` (`/ping`, `/devices`, `/devices/{guid}`, `POST /scan?...`),
collects the returned base64 JPEG pages and builds a PDF in the browser. No printing. mcprint the app replaces the helper and the page in one binary.
