// Generates the Homebrew formula (Linux) for a release. The macOS app is a cask, see make-cask.mjs.
//
//   node scripts/make-formula.mjs [version] > ../homebrew-tap/Formula/mcprint.rb
//
// The version defaults to the one in wails.json. The checksum is computed from the published tar.gz, not typed by hand.
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const version = process.argv[2]
  ?? JSON.parse(readFileSync(join(root, 'wails.json'), 'utf8')).info.productVersion;

const repo = 'mormitech/mcprint';
const url = `https://github.com/${repo}/releases/download/v${version}/mcprint-${version}-linux-x64.tar.gz`;
const res = await fetch(url);
if (!res.ok) throw new Error(`${url}: HTTP ${res.status} (is the release published?)`);
const sha = createHash('sha256').update(Buffer.from(await res.arrayBuffer())).digest('hex');

process.stdout.write(`class Mcprint < Formula
  desc "Scan pages of documents from network scanners (eSCL / AirScan)"
  homepage "https://github.com/${repo}"
  url "https://github.com/${repo}/releases/download/v${version}/mcprint-${version}-linux-x64.tar.gz"
  sha256 "${sha}"
  license "MIT"

  livecheck do
    url :url
    strategy :github_latest
  end

  # The macOS app is the cask of the same name: brew install --cask mormitech/tap/mcprint
  depends_on arch: :x86_64
  depends_on :linux

  def install
    bin.install "mcprint"
  end

  def caveats
    <<~EOS
      mcprint needs GTK 3 and WebKitGTK 4.1 from your system, for example on Debian and Ubuntu:
        sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
    EOS
  end

  test do
    assert_predicate bin/"mcprint", :executable?
  end
end
`);
