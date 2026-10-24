// Generates the Homebrew cask for a release.
//
//   node scripts/make-cask.mjs [version] > ../homebrew-tap/Casks/mcprint.rb        # release assets on GitHub
//   node scripts/make-cask.mjs --local dist > /tmp/mcprint.rb                       # local test: file:// zip in dist/
//
// The version defaults to the one in wails.json. The checksum is computed from the published zip, not typed by hand.
import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const root = resolve(import.meta.dirname, '..');
const args = process.argv.slice(2);
const localAt = args.indexOf('--local');
const localDir = localAt >= 0 ? resolve(args[localAt + 1]) : null;
const version = args.find((a, i) => !a.startsWith('--') && i !== localAt + 1)
  ?? JSON.parse(readFileSync(join(root, 'wails.json'), 'utf8')).info.productVersion;

const repo = 'mormitech/mcprint';
const file = `mcprint-${version}-macos-arm64.zip`;
const url = localDir ? pathToFileURL(join(localDir, file)).href : `https://github.com/${repo}/releases/download/v${version}/${file}`;

async function sha256(location) {
  let buf;
  if (location.startsWith('file:')) {
    if (!existsSync(new URL(location))) throw new Error(`${location} does not exist`);
    buf = readFileSync(new URL(location));
  } else {
    const res = await fetch(location);
    if (!res.ok) throw new Error(`${location}: HTTP ${res.status} (is the release published?)`);
    buf = Buffer.from(await res.arrayBuffer());
  }
  return createHash('sha256').update(buf).digest('hex');
}

// In a local test the url is a file:// one; a published cask always uses the #{version} form.
const urlLine = localDir ? url : `https://github.com/${repo}/releases/download/v#{version}/mcprint-#{version}-macos-arm64.zip`;

process.stdout.write(`cask "mcprint" do
  version "${version}"
  sha256 "${await sha256(url)}"

  url "${urlLine}"
  name "mcPrint"
  desc "Scan pages of documents from network scanners (eSCL / AirScan)"
  homepage "https://github.com/${repo}"

  livecheck do
    url :url
    strategy :github_latest
  end

  depends_on arch: :arm64
  depends_on macos: :monterey

  app "mcprint.app"

  # The app is not notarized yet, so Gatekeeper would block its first launch; clear the quarantine flag it got
  # when it was downloaded. (A third-party tap may do this; the main homebrew-cask repository does not allow it.)
  postflight_steps do
    run "/usr/bin/xattr",
        args:         ["-dr", "com.apple.quarantine", "{{appdir}}/mcprint.app"],
        must_succeed: false
  end

  zap trash: [
    "~/Library/Application Support/mcprint",
    "~/Library/Caches/mcprint",
  ]
end
`);
