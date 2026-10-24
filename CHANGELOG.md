# Changelog

All notable changes to mcPrint are listed here, newest first. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the versions are the [releases](https://github.com/mormitech/mcprint/releases).

## 0.8.0 – 2026-10-24

### Added
- The picture a scanner offers of itself is shown in the scanner list. A scanner that offers none looks as before.
- **Additional info** button in the scanner's details window. It opens a window with everything else the scanner will tell: links to its web
  pages, what it announced on the network, the jobs it remembers, and usage counters, consumables and error history where the scanner has
  them (read from Brother's and HP's well-known pages). A scanner that has none of it says so.
- Double-click a scanner in the list to open its details, like the **i** button.
- Demo mode (`--mock`): two of the made-up scanners offer a picture, one does not.

### Changed
- The details window lists only how many jobs the scanner keeps; the jobs themselves are in the additional info.
- The field for adding a scanner by address is at the bottom of the scanner list.

### Removed
- The **remove** button on a scanner in the list.
- The **Clear queue** button. Scanners keep their finished jobs as history and answer 404 when asked to delete one (a Brother does), so
  the button could not clear anything, and it reported jobs as cleared when it had not.

## 0.7.1 – 2026-10-24

### Fixed
- Dragging a page no longer flickers. The drop place is found from the row under the pointer and the pointer's position in it, not from
  the card nearest to the pointer, and a page that is still over its own place stays put.

## 0.7.0 – 2026-10-24

### Added
- A **+** between pages sets where the next scan goes; the dashed marker turns into the scanned page.
- Black & white on scanners that refuse it as JPEG (a Brother answers 409): the page is scanned in grey and made black and white here.
- The scanner's job queue is listed in its details, with a Clear queue button.
- Viewer: view-only rotation (**R** / **Shift+R**), nicer previous and next buttons.

### Changed
- A big **Scan** button below the settings. The success message goes away after a minute.
- A gap between the cards in the scanner list.
- A running scan can no longer be cancelled.
