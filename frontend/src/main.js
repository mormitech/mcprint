import './style.css';
import {
  Devices, Capabilities, Mode, ShowInfo, Search, AddDevice, RemoveDevice, Choices, Scan, CancelScan, Pages, DeletePage, MovePage,
  ClearPages, MovePageTo, ExportPDF, LoadPrefs, SavePrefs, Status,
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]));
const SOURCE = {platen: 'Flatbed', feeder: 'Feeder'};
const COLOR = {color: 'Colour', gray: 'Grey', bw: 'Black & white'};
// Icons for the source toggle: a flatbed with its lid raised over the glass, and a feeder with a sheet going in.
const ICON = {
  platen: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 11 7.5 5h12.5l-2.5 6"/><rect x="3" y="11" width="18" height="8" rx="2"/><path d="M6.5 15h11"/></svg>',
  feeder: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="2.5" width="8" height="7.5" rx="1"/><path d="M12 4.5v3.3m-1.7-1.6 1.7 1.7 1.7-1.7"/><rect x="3" y="12" width="18" height="8" rx="2"/><path d="M7 16h10"/></svg>',
};
const PAPER = {a4: 'A4', a5: 'A5', letter: 'Letter', legal: 'Legal', max: 'Largest'};

let devices = [];
let selected = null;          // guid
let choices = null;           // what the selected scanner offers for the current source
let opts = {source: '', duplex: false, dpi: 0, color: '', paper: 'a4'};
let pages = [];
let scan = {state: 'idle', message: ''};
let prefs = {last: '', devices: {}};
let ready = false;            // the capabilities of the selected scanner were loaded

const online = () => devices.find((d) => d.guid === selected)?.online;

async function savePrefs() {
  if (selected) prefs.devices[selected] = {...opts};
  prefs.last = selected || '';
  try { await SavePrefs(JSON.stringify(prefs)); } catch {}
}

// ---- Devices ---------------------------------------------------------------------------------------------

function ago(t) {
  const s = (Date.now() - new Date(t)) / 1000;
  if (!(s >= 0)) return '';
  return s < 90 ? 'just now' : s < 5400 ? `${Math.round(s / 60)} min ago` : s < 129600 ? `${Math.round(s / 3600)} h ago` : `${Math.round(s / 86400)} days ago`;
}

function renderDevices() {
  const el = $('devices');
  if (!devices.length) {
    el.innerHTML = '<p class="muted">Looking for scanners on the network… Scanners must support eSCL / AirScan. If yours does not show up, add it by address.</p>';
    return;
  }
  el.innerHTML = devices.map((d) => `
    <div class="device ${d.guid === selected ? 'sel' : ''} ${d.online ? '' : 'off'}" data-guid="${esc(d.guid)}">
      <span class="dot"></span>
      <span class="name">${esc(d.name)}</span>
      <span class="sub">${esc(d.host)}${d.port && d.port !== 80 ? ':' + d.port : ''}${d.scheme === 'https' ? ' · https' : ''}${d.manual ? ' · added by hand' : ''}</span>
      <span class="sub">${d.online ? 'online' : 'offline, last seen ' + ago(d.lastSeen)}</span>
      <button class="rm" data-rm="${esc(d.guid)}" title="Forget this scanner">remove</button>
    </div>`).join('');
}

async function setDevices(list) {
  devices = list || [];
  renderDevices();
  if (!selected && devices.length) {
    const want = devices.find((d) => d.guid === prefs.last) || devices.find((d) => d.online);
    if (want) await select(want.guid);
  } else {
    renderControls(); // online/offline may have changed
  }
}

async function select(guid) {
  selected = guid;
  ready = false;
  choices = null;
  scan = {state: 'idle', message: ''};
  renderDevices();
  renderControls();
  const saved = prefs.devices[guid] || {};
  opts = {source: saved.source || '', duplex: !!saved.duplex, dpi: saved.dpi || 0, color: saved.color || '', paper: saved.paper || 'a4'};
  try {
    await loadChoices();
  } catch (e) {
    if (selected === guid) { scan = {state: 'error', message: String(e)}; }
  }
  renderControls();
  savePrefs();
}

// Asks the backend what the scanner offers for the chosen source and fixes up values that do not apply.
async function loadChoices() {
  const guid = selected;
  let c = await Choices(guid, opts.source, opts.duplex);
  if (guid !== selected) return;
  if (!c.sources.includes(opts.source)) opts.source = c.sources.includes('feeder') ? 'feeder' : c.sources[0] || '';
  if (opts.duplex && !c.canDuplex) opts.duplex = false;
  c = await Choices(guid, opts.source, opts.duplex);
  if (guid !== selected) return;
  if (!c.colors.includes(opts.color)) opts.color = c.colors[0] || '';
  if (!c.dpis.includes(opts.dpi) && c.dpis.length) opts.dpi = c.dpis.includes(c.defaultDpi) ? c.defaultDpi : c.dpis[0];
  if (!c.papers.includes(opts.paper)) opts.paper = 'a4';
  choices = c;
  ready = true;
}

// ---- Scan form -------------------------------------------------------------------------------------------

function opt(map, value, current) {
  return `<option value="${esc(value)}" ${value === current ? 'selected' : ''}>${esc(map[value] ?? value)}</option>`;
}

function renderControls() {
  const el = $('controls');
  const d = devices.find((x) => x.guid === selected);
  if (!d) {
    el.innerHTML = '<p class="empty">Select a scanner on the left to scan.</p>';
    return;
  }
  const busy = scan.state === 'scanning';
  let form = '';
  if (ready && choices) {
    form = `<div class="form">
      <div class="field">Source<div class="seg" role="radiogroup" aria-label="Source">${choices.sources.map((s) =>
        `<button type="button" role="radio" aria-checked="${s === opts.source}" class="${s === opts.source ? 'on' : ''}" data-source="${s}" ${busy || choices.sources.length < 2 ? 'disabled' : ''}>${ICON[s]}<span>${SOURCE[s]}</span></button>`).join('')}</div></div>
      ${choices.canDuplex && opts.source === 'feeder' ? `<label class="check"><input type="checkbox" id="o-duplex" ${opts.duplex ? 'checked' : ''} ${busy ? 'disabled' : ''}> Both sides</label>` : ''}
      <label>Colour<select id="o-color" ${busy ? 'disabled' : ''}>${choices.colors.map((c) => opt(COLOR, c, opts.color)).join('')}</select></label>
      <label>Resolution<select id="o-dpi" ${busy ? 'disabled' : ''}>${choices.dpis.map((v) => `<option value="${v}" ${v === opts.dpi ? 'selected' : ''}>${v} dpi</option>`).join('')}</select></label>
      <label>Paper<select id="o-paper" ${busy ? 'disabled' : ''}>${choices.papers.map((p) => opt(PAPER, p, opts.paper)).join('')}</select></label>
      ${busy
        ? '<button id="cancel" class="danger">Cancel</button>'
        : `<button id="scan" class="primary tip" data-tip="${pages.length ? `Adds page ${pages.length + 1} to this document` : 'Starts a new document'}" ${d.online ? '' : 'disabled'}>${pages.length ? '+ ' : ''}Scan</button>`}
    </div>`;
  } else if (!scan.message) {
    form = '<p class="empty">Reading what the scanner can do…</p>';
  }
  const hint = !d.online && !busy ? '<div class="state error">This scanner is offline right now.</div>' : '';
  const st = scan.message ? `<div class="state ${scan.state}">${esc(scan.message)}</div>` : '';
  el.innerHTML = `<div class="row"><strong>${esc(d.name)}</strong><button id="info-btn" class="icon tip tip-left" data-tip="Scanner details" aria-label="Scanner details">i</button></div><div style="height:10px"></div>${form}${hint}${st}`;
}

$('controls').addEventListener('change', async (ev) => {
  const t = ev.target;
  if (t.id === 'o-duplex') opts.duplex = t.checked;
  else if (t.id === 'o-color') opts.color = t.value;
  else if (t.id === 'o-dpi') opts.dpi = Number(t.value);
  else if (t.id === 'o-paper') opts.paper = t.value;
  else return;
  if (t.id === 'o-duplex') {
    try { await loadChoices(); } catch (e) { scan = {state: 'error', message: String(e)}; }
  }
  renderControls();
  savePrefs();
});

$('controls').addEventListener('click', async (ev) => {
  const src = ev.target.closest('[data-source]');
  if (src && src.dataset.source !== opts.source) {
    opts.source = src.dataset.source;
    opts.duplex = false;
    try { await loadChoices(); } catch (e) { scan = {state: 'error', message: String(e)}; }
    renderControls();
    savePrefs();
    return;
  }
  if (ev.target.id === 'cancel') { CancelScan(); return; }
  if (ev.target.id !== 'scan') return;
  scan = {state: 'scanning', message: 'Starting…'};
  renderControls();
  try {
    await Scan({guid: selected, ...opts});
  } catch (e) {
    scan = {state: 'error', message: String(e)};
    renderControls();
  }
});

// ---- Pages -----------------------------------------------------------------------------------------------

function renderPages() {
  cancelDrag();
  renderControls();
  const bar = $('pages-bar');
  bar.innerHTML = pages.length
    ? `<span class="grow">${pages.length} page${pages.length === 1 ? '' : 's'}</span>
       <button id="pdf" class="primary" ${scan.state === 'scanning' ? 'disabled' : ''}>Save as PDF…</button>
       <button id="clear" class="danger" ${scan.state === 'scanning' ? 'disabled' : ''}>Clear all</button>`
    : '';
  $('pages').innerHTML = pages.map((p, i) => `
    <div class="page" data-id="${p.id}">
      <img src="/page/${p.id}.jpg" alt="Page ${i + 1}" data-view="${p.id}" loading="lazy" draggable="false">
      <div class="tools">
        <span><span class="n">${i + 1}</span> · ${p.width}×${p.height}</span>
        <button data-move="-1" ${i === 0 ? 'disabled' : ''} title="Move earlier">←</button>
        <button data-move="1" ${i === pages.length - 1 ? 'disabled' : ''} title="Move later">→</button>
        <button data-del="1" class="danger" title="Delete page">✕</button>
      </div>
    </div>`).join('');
}

$('pages').addEventListener('click', async (ev) => {
  if (justDragged) { justDragged = false; return; } // the click that ends a drag is not a click
  const t = ev.target;
  const id = Number(t.closest('.page')?.dataset.id);
  if (t.dataset.view) { $('viewer').innerHTML = `<img src="/page/${t.dataset.view}.jpg">`; $('viewer').hidden = false; }
  else if (t.dataset.del) { pages = await DeletePage(id); renderPages(); }
  else if (t.dataset.move) { pages = await MovePage(id, Number(t.dataset.move)); renderPages(); }
});
$('viewer').addEventListener('click', () => { $('viewer').hidden = true; });
document.addEventListener('keydown', (ev) => { if (ev.key === 'Escape') $('viewer').hidden = true; });

$('pages-bar').addEventListener('click', async (ev) => {
  if (ev.target.id === 'clear') { pages = await ClearPages(); renderPages(); }
  if (ev.target.id === 'pdf') {
    try {
      const path = await ExportPDF();
      if (path) { scan = {state: 'done', message: `Saved ${path}`}; renderControls(); }
    } catch (e) { scan = {state: 'error', message: String(e)}; renderControls(); }
  }
});

// ---- Add / search ----------------------------------------------------------------------------------------

$('search').addEventListener('click', () => {
  manual = {started: Date.now(), seen: false};
  $('search').disabled = true;
  $('search').classList.add('spin');
  setTimeout(() => { if (manual && Date.now() - manual.started >= 15000) endManual(); }, 15000);
  Search();
});
$('devices').addEventListener('click', async (ev) => {
  const rm = ev.target.dataset.rm;
  if (rm) {
    await RemoveDevice(rm);
    if (rm === selected) { selected = null; ready = false; }
    setDevices(await Devices());
    return;
  }
  const guid = ev.target.closest('.device')?.dataset.guid;
  if (guid && guid !== selected) select(guid);
});
$('add').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  const addr = $('addr').value.trim();
  if (!addr) return;
  $('addmsg').textContent = 'Trying…';
  try {
    const d = await AddDevice(addr);
    $('addmsg').textContent = '';
    $('addr').value = '';
    await setDevices(await Devices());
    select(d.guid);
  } catch (e) {
    $('addmsg').textContent = String(e);
  }
});

// ---- Start -----------------------------------------------------------------------------------------------

EventsOn('devices', (list) => setDevices(list));
// After a click on the refresh button the icon spins and the button is disabled until that search has ended
// (at least a moment, so a click is always visible). Searches in the background do not touch the button.
let manual = null;            // {started: ms, seen: a search started after the click}
const endManual = () => { manual = null; $('search').disabled = false; $('search').classList.remove('spin'); };
EventsOn('searching', (on) => {
  if (!manual) return;
  if (on) { manual.seen = true; return; }
  if (manual.seen) setTimeout(endManual, Math.max(0, manual.started + 1000 - Date.now()));
});
EventsOn('pages', (list) => { pages = list || []; renderPages(); });
EventsOn('scan', (ev) => { scan = ev; renderControls(); renderPages(); });

(async () => {
  const infoGuid = await Mode();
  if (infoGuid) { showDetailsWindow(infoGuid); return; }
  try { prefs = {last: '', devices: {}, ...JSON.parse(await LoadPrefs())}; } catch {}
  pages = (await Pages()) || [];
  renderPages();
  renderControls();
  await setDevices(await Devices());
})();

// ---- Scanner details -------------------------------------------------------------------------------------

const mm = (v) => Math.round(v / 300 * 25.4);
const list = (a) => (a && a.length ? a.map(esc).join(', ') : '–');
const dpiText = (r) => (r.values?.length ? r.values.join(', ') : r.max ? `${r.min}–${r.max} (step ${r.step})` : '–');

function inputRows(name, i) {
  if (!i) return `<tr><th>${name}</th><td>not available</td></tr>`;
  return `<tr><th>${name}</th><td>
    Colour: ${list(i.colorModes.map((c) => ({RGB24: 'colour', Grayscale8: 'grey', BlackAndWhite1: 'black & white'}[c] || c)))}<br>
    Resolution: ${esc(dpiText(i.resolutions))} dpi<br>
    Formats: ${list(i.formats)}<br>
    Largest page: ${i.maxWidth ? `${mm(i.maxWidth)} × ${mm(i.maxHeight)} mm` : '–'}</td></tr>`;
}

function renderDetails(d, caps, status, err) {
  const base = `${d.scheme}://${d.host.includes(':') ? `[${d.host}]` : d.host}:${d.port}${d.root ? '/' + d.root : ''}`;
  const alive = status ? 'online' : 'not reachable right now';
  $('detail').innerHTML = `<h3>${esc(d.name)}</h3>
      <h4>Found on the network</h4>
      <table>
        <tr><th>IP address</th><td>${esc(d.host)}</td></tr>
        <tr><th>Port</th><td>${d.port}</td></tr>
        <tr><th>Protocol</th><td>${esc(d.scheme)}${d.scheme === 'https' ? ' (certificate not verified)' : ''}</td></tr>
        <tr><th>eSCL path</th><td>/${esc(d.root)}</td></tr>
        <tr><th>Address</th><td>${esc(base)}</td></tr>
        <tr><th>Model (mDNS)</th><td>${esc(d.model) || '–'}</td></tr>
        <tr><th>Scanner ID</th><td>${esc(d.guid)}</td></tr>
        <tr><th>Found by</th><td>${d.manual ? 'added by address' : 'mDNS (Bonjour)'}</td></tr>
        <tr><th>State</th><td>${alive}, last seen ${esc(new Date(d.lastSeen).toLocaleString())}</td></tr>
      </table>
      <h4>Reported by the scanner</h4>
      ${err ? `<p class="muted">${esc(err)}</p>` : !caps ? '<p class="muted">Reading…</p>' : `<table>
        <tr><th>Make and model</th><td>${esc([caps.make, caps.model].filter(Boolean).join(' ')) || '–'}</td></tr>
        <tr><th>eSCL version</th><td>${esc(caps.version) || '–'}</td></tr>
        <tr><th>UUID</th><td>${esc(caps.uuid) || '–'}</td></tr>
        ${status ? `<tr><th>Status</th><td>${esc(status.state)}${status.adfState ? `, feeder: ${esc(status.adfState.replace('ScannerAdf', '').toLowerCase())}` : ''}</td></tr>` : ''}
        ${caps.feederCapacity ? `<tr><th>Feeder capacity</th><td>${caps.feederCapacity} sheets</td></tr>` : ''}
        ${inputRows('Flatbed', caps.platen)}${inputRows('Feeder, one side', caps.simplex)}${inputRows('Feeder, both sides', caps.duplex)}
      </table>`}`;
}

// The details window (the program started with --info) shows this instead of the app.
async function showDetailsWindow(guid) {
  document.body.classList.add('detail-mode');
  $('detail').hidden = false;
  const d = (await Devices()).find((x) => x.guid === guid);
  if (!d) { $('detail').innerHTML = '<p class="muted">This scanner is not known (any more).</p>'; return; }
  renderDetails(d);
  try {
    const [caps, status] = await Promise.all([Capabilities(guid), Status(guid).catch(() => null)]);
    renderDetails(d, caps, status);
  } catch (e) {
    renderDetails(d, null, null, String(e));
  }
}

$('controls').addEventListener('click', async (ev) => {
  if (!ev.target.closest('#info-btn')) return;
  try { await ShowInfo(selected); } catch (e) { scan = {state: 'error', message: String(e)}; renderControls(); }
});

// ---- Drag and drop to rearrange --------------------------------------------------------------------------
// Pointer events instead of HTML5 drag and drop, so it behaves the same in every platform's webview. While dragging,
// a copy of the page follows the pointer and the page itself becomes a dashed placeholder that moves through the
// grid: that is where the page will land when it is dropped.

let drag = null;            // {card, id, startX, startY, ghost, offX, offY, active, from}
let justDragged = false;

function cancelDrag(restore = true) {
  if (!drag) return;
  drag.ghost?.remove();
  drag.card.classList.remove('placeholder');
  document.body.classList.remove('dragging');
  clearInterval(drag.scroller);
  if (restore && drag.active) { drag = null; renderPages(); return; }
  drag = null;
}

function renumber() {
  [...$('pages').children].forEach((c, i) => { const n = c.querySelector('.n'); if (n) n.textContent = i + 1; });
}

// Moves the placeholder to where the pointer says, animating the pages that make room (FLIP).
function placeAt(x, y) {
  const grid = $('pages');
  const others = [...grid.children].filter((c) => c !== drag.card);
  if (!others.length) return;
  let best = null, bestD = Infinity;
  for (const c of others) {
    const r = c.getBoundingClientRect();
    const dx = Math.max(r.left - x, 0, x - r.right), dy = Math.max(r.top - y, 0, y - r.bottom);
    const d = dx * dx + dy * dy;
    if (d < bestD) { bestD = d; best = c; }
  }
  const r = best.getBoundingClientRect();
  // in the same row: left half is before, right half is after; a pointer below the row is after, above is before
  const after = y > r.bottom ? true : y < r.top ? false : x > r.left + r.width / 2;
  const ref = after ? best.nextElementSibling : best;
  if (ref === drag.card) return;
  if (drag.card.nextElementSibling === ref) return; // already there
  const before = new Map([...grid.children].map((c) => [c, c.getBoundingClientRect()]));
  grid.insertBefore(drag.card, ref);
  for (const c of grid.children) {
    if (c === drag.card) continue;
    const a = before.get(c), b = c.getBoundingClientRect();
    const dx = a.left - b.left, dy = a.top - b.top;
    if (!dx && !dy) continue;
    c.style.transition = 'none';
    c.style.transform = `translate(${dx}px, ${dy}px)`;
    requestAnimationFrame(() => {
      c.style.transition = 'transform .18s ease';
      c.style.transform = '';
    });
  }
  renumber();
}

$('pages').addEventListener('pointerdown', (ev) => {
  const card = ev.target.closest('.page');
  if (!card || ev.button !== 0 || ev.target.closest('button') || scan.state === 'scanning') return;
  drag = {card, id: Number(card.dataset.id), startX: ev.clientX, startY: ev.clientY, active: false};
});

window.addEventListener('pointermove', (ev) => {
  if (!drag) return;
  if (!drag.active) {
    if (Math.hypot(ev.clientX - drag.startX, ev.clientY - drag.startY) < 6) return;
    const r = drag.card.getBoundingClientRect();
    drag.active = true;
    drag.offX = drag.startX - r.left;
    drag.offY = drag.startY - r.top;
    drag.ghost = drag.card.cloneNode(true);
    drag.ghost.classList.add('ghost');
    drag.ghost.style.width = `${r.width}px`;
    document.body.appendChild(drag.ghost);
    drag.card.classList.add('placeholder');
    document.body.classList.add('dragging');
    // scroll the page area while the pointer is near its top or bottom edge
    drag.scroller = setInterval(() => {
      const m = document.querySelector('main').getBoundingClientRect();
      const y = drag.y ?? 0;
      const speed = y < m.top + 50 ? -14 : y > m.bottom - 50 ? 14 : 0;
      if (speed) { document.querySelector('main').scrollBy(0, speed); placeAt(drag.x, drag.y); }
    }, 30);
  }
  drag.x = ev.clientX; drag.y = ev.clientY;
  drag.ghost.style.transform = `translate(${ev.clientX - drag.offX}px, ${ev.clientY - drag.offY}px) rotate(2deg)`;
  placeAt(ev.clientX, ev.clientY);
});

window.addEventListener('pointerup', async () => {
  if (!drag) return;
  if (!drag.active) { drag = null; return; }
  const id = drag.id;
  const index = [...$('pages').children].indexOf(drag.card);
  justDragged = true;
  setTimeout(() => { justDragged = false; }, 0); // only the click straight after the drop is swallowed
  cancelDrag(false);
  pages = await MovePageTo(id, index);
  renderPages();
});

window.addEventListener('keydown', (ev) => { if (ev.key === 'Escape') cancelDrag(); });
window.addEventListener('blur', () => cancelDrag());
