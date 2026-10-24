import './style.css';
import {
  Version, Devices, Capabilities, Mode, ShowInfo, Search, AddDevice, RemoveDevice, Choices, Scan, CancelScan, Pages, DeletePage, MovePage,
  ClearPages, MovePageTo, ExportPDF, LoadPrefs, SavePrefs, Status,
} from '../wailsjs/go/main/App';
import {EventsOn, BrowserOpenURL} from '../wailsjs/runtime/runtime';
import logo from './assets/mormitech-mark.svg';

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]));
const SOURCE = {platen: 'Flatbed', feeder: 'Feeder'};
const COLOR = {color: 'Colour', gray: 'Grey', bw: 'Black & white'};
// Icons for the source toggle: a flatbed with its lid raised over the glass, and a feeder with a sheet going in.
const ICON = {
  platen: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 11 7.5 5h12.5l-2.5 6"/><rect x="3" y="11" width="18" height="8" rx="2"/><path d="M6.5 15h11"/></svg>',
  feeder: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="2.5" width="8" height="7.5" rx="1"/><path d="M12 4.5v3.3m-1.7-1.6 1.7 1.7 1.7-1.7"/><rect x="3" y="12" width="18" height="8" rx="2"/><path d="M7 16h10"/></svg>',
};
const GLYPH = {
  left: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m14.5 6-6 6 6 6"/></svg>',
  right: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9.5 6 6 6-6 6"/></svg>',
  trash: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4.5 7h15M10 7V4.5h4V7M6.5 7l.8 12.5h9.4L17.5 7M10 11v5.5M14 11v5.5"/></svg>',
};
const PAPER = {a4: 'A4', a5: 'A5', letter: 'Letter', legal: 'Legal', max: 'Largest'};

let devices = [];
let selected = null;          // guid
let choices = null;           // what the selected scanner offers for the current source
let opts = {source: '', duplex: false, dpi: 0, color: '', paper: 'a4'};
let pages = [];
let scan = {state: 'idle', message: ''};

// Wails rejects with an Error whose text starts with "Error: "; show a plain sentence instead.
const errText = (e) => {
  const t = String(e?.message ?? e).replace(/^Error:\s*/, '');
  return t.charAt(0).toUpperCase() + t.slice(1);
};
// An error the user can try again: retry is 'connect' (reading the scanner) or 'scan' (the scan itself).
const fail = (e, retry) => ({state: 'error', message: errText(e), retry});
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
    if (selected === guid) scan = fail(e, 'connect');
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

// A row of buttons of which one is selected. icon is optional; a lone option is shown but cannot be changed.
function seg(label, key, items, current, disabled) {
  return `<div class="field">${label}<div class="seg" role="radiogroup" aria-label="${label}">${items.map((it) =>
    `<button type="button" role="radio" aria-checked="${it.value === current}" class="${it.value === current ? 'on' : ''}" data-seg="${key}" data-value="${esc(it.value)}" ${disabled || items.length < 2 ? 'disabled' : ''}>${it.icon || ''}<span>${esc(it.label)}</span></button>`).join('')}</div></div>`;
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
      ${seg('Source', 'source', choices.sources.map((v) => ({value: v, label: SOURCE[v], icon: ICON[v]})), opts.source, busy)}
      ${choices.canDuplex && opts.source === 'feeder' ? `<label class="check"><input type="checkbox" id="o-duplex" ${opts.duplex ? 'checked' : ''} ${busy ? 'disabled' : ''}> Both sides</label>` : ''}
      ${seg('Colour', 'color', choices.colors.map((v) => ({value: v, label: COLOR[v] ?? v})), opts.color, busy)}
      ${choices.dpis.length <= 6
        ? seg('Resolution', 'dpi', choices.dpis.map((v) => ({value: String(v), label: `${v} dpi`})), String(opts.dpi), busy)
        : `<label>Resolution<select id="o-dpi" ${busy ? 'disabled' : ''}>${choices.dpis.map((v) => `<option value="${v}" ${v === opts.dpi ? 'selected' : ''}>${v} dpi</option>`).join('')}</select></label>`}
      ${seg('Paper', 'paper', choices.papers.map((v) => ({value: v, label: PAPER[v] ?? v})), opts.paper, busy)}
      ${busy
        ? '<button id="cancel" class="danger">Cancel</button>'
        : `<button id="scan" class="primary tip" data-tip="${pages.length ? `Adds page ${pages.length + 1} to this document` : 'Starts a new document'}" ${d.online ? '' : 'disabled'}>${pages.length ? '+ ' : ''}Scan</button>`}
    </div>`;
  } else if (!scan.message) {
    form = '<p class="empty">Reading what the scanner can do…</p>';
  }
  const hint = !d.online && !busy ? '<div class="state error">This scanner is offline right now.</div>' : '';
  const st = scan.message ? `<div class="state ${scan.state}">${esc(scan.message)}${scan.state === 'error' && scan.retry ? ` <button id="retry" class="retry">${scan.retry === 'scan' ? 'Try again' : 'Reconnect'}</button>` : ''}</div>` : '';
  el.innerHTML = `<div class="row"><strong>${esc(d.name)}</strong><button id="info-btn" class="icon tip tip-left" data-tip="Scanner details" aria-label="Scanner details">i</button></div><div style="height:10px"></div>${form}${hint}${st}`;
}

$('controls').addEventListener('change', async (ev) => {
  const t = ev.target;
  if (t.id === 'o-duplex') opts.duplex = t.checked;
  else if (t.id === 'o-dpi') opts.dpi = Number(t.value);
  else return;
  if (t.id === 'o-duplex') {
    try { await loadChoices(); } catch (e) { scan = fail(e, 'connect'); }
  }
  renderControls();
  savePrefs();
});

$('controls').addEventListener('click', async (ev) => {
  const sg = ev.target.closest('[data-seg]');
  if (sg) {
    const {seg: key, value} = sg.dataset;
    if (key === 'dpi') opts.dpi = Number(value);
    else if (opts[key] === value) return;
    else opts[key] = value;
    if (key === 'source') {
      opts.duplex = false;
      try { await loadChoices(); } catch (e) { scan = fail(e, 'connect'); }
    }
    renderControls();
    savePrefs();
    return;
  }
  if (ev.target.id === 'cancel') { CancelScan(); return; }
  if (ev.target.id === 'retry') { scan.retry === 'scan' ? startScan() : retryConnect(); return; }
  if (ev.target.id !== 'scan') return;
  startScan();
});

async function startScan() {
  scan = {state: 'scanning', message: 'Starting…'};
  renderControls();
  try {
    await Scan({guid: selected, ...opts});
  } catch (e) {
    scan = fail(e, 'scan');
    renderControls();
  }
}

// Reconnect: search the network again and read the scanner's capabilities once more.
async function retryConnect() {
  Search();
  await select(selected);
}

// ---- Pages -----------------------------------------------------------------------------------------------

// The line above the pages: how many, and what can be done with them.
function renderBar() {
  const bar = $('pages-bar');
  bar.innerHTML = pages.length
    ? `<span class="grow">${pages.length} page${pages.length === 1 ? '' : 's'}</span>
       <button id="pdf" class="primary" ${scan.state === 'scanning' ? 'disabled' : ''}>Save as PDF…</button>
       <button id="clear" class="danger" ${scan.state === 'scanning' ? 'disabled' : ''}>Clear all</button>`
    : '';
}

function renderPages() {
  cancelDrag();
  renderControls();
  renderBar();
  $('pages').innerHTML = pages.map((p, i) => `
    <div class="page" data-id="${p.id}">
      <img src="/page/${p.id}.jpg" alt="Page ${i + 1}" data-view="${p.id}" loading="lazy" draggable="false">
      <div class="tools">
        <span class="label"><b class="n">${i + 1}</b><small>${p.width}×${p.height}</small></span>
        <button class="ic" data-move="-1" ${i === 0 ? 'disabled' : ''} title="Move earlier" aria-label="Move earlier">${GLYPH.left}</button>
        <button class="ic" data-move="1" ${i === pages.length - 1 ? 'disabled' : ''} title="Move later" aria-label="Move later">${GLYPH.right}</button>
        <button class="ic danger" data-del="1" title="Delete page" aria-label="Delete page">${GLYPH.trash}</button>
      </div>
    </div>`).join('');
}

$('pages').addEventListener('click', async (ev) => {
  if (justDragged) { justDragged = false; return; } // the click that ends a drag is not a click
  const t = ev.target.closest('[data-view], [data-del], [data-move]') || ev.target;
  const id = Number(t.closest('.page')?.dataset.id);
  if (t.dataset.view) openViewer(Number(t.dataset.view), t);
  else if (t.dataset.del) deleteCard(id);
  else if (t.dataset.move) moveCard(id, Number(t.dataset.move));
});

$('pages-bar').addEventListener('click', async (ev) => {
  if (ev.target.id === 'clear') { pages = await ClearPages(); renderPages(); }
  if (ev.target.id === 'pdf') {
    try {
      const path = await ExportPDF();
      if (path) { scan = {state: 'done', message: `Saved ${path}`}; renderControls(); }
    } catch (e) { scan = fail(e); renderControls(); }
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
    $('addmsg').textContent = errText(e);
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
EventsOn('scan', (ev) => { scan = ev.state === 'error' ? {...ev, retry: 'scan'} : ev; renderControls(); renderPages(); });

(async () => {
  const mode = await Mode();
  if (mode === 'about') { showAbout(); return; }
  if (mode.startsWith('info:')) { showDetailsWindow(mode.slice(5)); return; }
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

// Most scanners already put their maker in the model name ("Brother MFC-L2710DN series"): do not say it twice.
function makeAndModel(c) {
  const make = (c.make || '').trim(), model = (c.model || '').trim();
  return model.toLowerCase().startsWith(make.toLowerCase()) ? model : `${make} ${model}`.trim();
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
        <tr><th>Make and model</th><td>${esc(makeAndModel(caps)) || '–'}</td></tr>
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
    renderDetails(d, null, null, errText(e));
  }
}

$('controls').addEventListener('click', async (ev) => {
  if (!ev.target.closest('#info-btn')) return;
  try { await ShowInfo(selected); } catch (e) { scan = fail(e); renderControls(); }
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

// Numbers the pages as they stand now; the page in the air gets the number it would have if dropped here.
function renumber() {
  [...$('pages').children].forEach((c, i) => {
    const n = c.querySelector('.n');
    if (n) n.textContent = i + 1;
    if (c === drag?.card) {
      const g = drag.ghost?.querySelector('.n');
      if (g) g.textContent = i + 1;
    }
  });
}

// FLIP: runs mutate(), which reorders the cards, and slides every card from where it was to where it is now.
// The card in `lift` travels slightly raised and bigger, so it is clear which one moved.
function flip(mutate, lift) {
  const grid = $('pages');
  const before = new Map([...grid.children].map((c) => [c, c.getBoundingClientRect()]));
  mutate();
  for (const c of grid.children) {
    const a = before.get(c), b = c.getBoundingClientRect();
    const dx = a.left - b.left, dy = a.top - b.top;
    if (!dx && !dy) continue;
    const up = c === lift;
    c.getAnimations().forEach((an) => an.cancel());
    if (up) c.style.zIndex = 5;
    const anim = c.animate(
      up
        ? [{transform: `translate(${dx}px, ${dy}px) scale(1)`}, {transform: `translate(${dx / 2}px, ${dy / 2}px) scale(1.06)`, offset: .5}, {transform: 'none'}]
        : [{transform: `translate(${dx}px, ${dy}px)`}, {transform: 'none'}],
      {duration: up ? 340 : 260, easing: 'cubic-bezier(.2,.8,.2,1)'});
    if (up) anim.onfinish = anim.oncancel = () => { c.style.zIndex = ''; };
  }
}

// Keeps the first card's "earlier" and the last card's "later" button disabled.
function updateArrows() {
  const cards = [...$('pages').children];
  cards.forEach((c, i) => {
    c.querySelector('[data-move="-1"]').disabled = i === 0;
    c.querySelector('[data-move="1"]').disabled = i === cards.length - 1;
  });
}

// The trash button: the page shrinks away, the ones behind it slide forward to close the gap.
async function deleteCard(id) {
  const grid = $('pages');
  const card = grid.querySelector(`.page[data-id="${id}"]`);
  if (!card || card.dataset.leaving) return;
  card.dataset.leaving = '1';
  card.style.pointerEvents = 'none';
  const backend = DeletePage(id);
  await card.animate(
    [{opacity: 1, transform: 'scale(1)'}, {opacity: 0, transform: 'scale(.8)'}],
    {duration: 200, easing: 'ease-in', fill: 'forwards'}).finished.catch(() => {});
  flip(() => card.remove());
  renumber();
  updateArrows();
  try { pages = await backend; } catch { pages = await Pages(); renderPages(); return; }
  renderBar();
  renderControls();
}

// The arrow buttons: the page glides to its new place at once, the backend catches up.
async function moveCard(id, delta) {
  const grid = $('pages');
  const card = grid.querySelector(`.page[data-id="${id}"]`);
  const other = delta < 0 ? card?.previousElementSibling : card?.nextElementSibling;
  if (!card || !other) return;
  flip(() => grid.insertBefore(card, delta < 0 ? other : other.nextElementSibling), card);
  renumber();
  updateArrows();
  try { pages = await MovePage(id, delta); } catch { pages = await Pages(); renderPages(); }
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
  flip(() => grid.insertBefore(drag.card, ref));
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
    drag.ghost.querySelector('.n')?.classList.add('flying');
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

// ---- About window ----------------------------------------------------------------------------------------
// The program started with --about shows this. Links open in the system's browser.

const COFFEE = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4.5 9.5h12v5a5 5 0 0 1-5 5h-2a5 5 0 0 1-5-5zM16.5 11h1.5a2.5 2.5 0 0 1 0 5h-1.8M8 3.5c-.9 1 .9 1.8 0 2.8M12 3.5c-.9 1 .9 1.8 0 2.8"/></svg>';
const REPO = 'https://github.com/mormitech/mcprint';

async function showAbout() {
  document.body.classList.add('about-mode');
  $('about').hidden = false;
  $('about').innerHTML = `<img class="logo" src="${logo}" alt="" width="132">
    <h1>mcPrint</h1>
    <p class="muted">Version ${esc(await Version())}</p>
    <p>Scan pages of documents from network scanners (eSCL / AirScan) on macOS, Windows and Linux.</p>
    <p class="muted">Free and open source, MIT license. No accounts, no ads, no telemetry.</p>
    <p class="links"><a href="${REPO}">Website and source code</a><a href="${REPO}/issues">Report a problem</a><a href="${REPO}/blob/main/LICENSE">License</a><a class="coffee" href="https://mormitech.com/support-us/">${COFFEE}Buy me a coffee</a></p>`;
}

document.addEventListener('click', (ev) => {
  const a = ev.target.closest('a[href^="http"]');
  if (a) { ev.preventDefault(); BrowserOpenURL(a.href); }
});

// ---- Page viewer -----------------------------------------------------------------------------------------
// Opens over the window when a page is clicked. Scroll wheel (or a pinch on the trackpad) zooms around the pointer,
// dragging moves the page, a double click toggles between fit and zoomed, and the arrow keys flip through the pages.

const VIEW_ICON = {
  left: GLYPH.left, right: GLYPH.right,
  close: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6.5 6.5 11 11m0-11-11 11"/></svg>',
  plus: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5.5v13M5.5 12h13"/></svg>',
  minus: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5.5 12h13"/></svg>',
  fit: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4.5 9V4.5H9M15 4.5h4.5V9M19.5 15v4.5H15M9 19.5H4.5V15"/></svg>',
};

let view = null; // {idx, s, x, y, fit, nw, nh, drag, hintTimer}

const vStage = () => $('viewer').querySelector('.stage');
const vImg = () => $('viewer').querySelector('.stage img');
const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

function viewerBounds() {
  const r = vStage().getBoundingClientRect();
  return {w: r.width, h: r.height, left: r.left, top: r.top};
}

// Keeps the page on the screen: smaller than the window it is centred, bigger it cannot leave a gap.
function constrain() {
  const b = viewerBounds();
  const w = view.nw * view.s, h = view.nh * view.s;
  view.x = w <= b.w ? (b.w - w) / 2 : clamp(view.x, b.w - w, 0);
  view.y = h <= b.h ? (b.h - h) / 2 : clamp(view.y, b.h - h, 0);
}

function applyView(smooth) {
  const img = vImg();
  img.style.transition = smooth ? 'transform .22s cubic-bezier(.2,.8,.2,1)' : 'none';
  img.style.transform = `translate(${view.x}px, ${view.y}px) scale(${view.s})`;
  const pct = Math.round(view.s / view.fit * 100);
  $('viewer').querySelector('.pct').textContent = `${pct}%`;
  $('viewer').classList.toggle('zoomed', view.s > view.fit * 1.001);
}

const maxScale = () => Math.max(view.fit * 12, 1.5);
const minScale = () => view.fit * 0.6;

// Zooms by a factor, keeping the point (px, py), in stage coordinates, where it is.
function zoomBy(factor, px, py, smooth) {
  const b = viewerBounds();
  px ??= b.w / 2; py ??= b.h / 2;
  const s = clamp(view.s * factor, minScale(), maxScale());
  const k = s / view.s;
  view.x = px - (px - view.x) * k;
  view.y = py - (py - view.y) * k;
  view.s = s;
  constrain();
  applyView(smooth);
}

function fitView(smooth) {
  const b = viewerBounds();
  view.fit = Math.min((b.w - 120) / view.nw, (b.h - 150) / view.nh);
  view.s = view.fit;
  view.x = view.y = 0;
  constrain();
  applyView(smooth);
}

function showPage(idx, from) {
  const p = pages[idx];
  view.idx = idx;
  view.nw = p.width; view.nh = p.height;
  const img = vImg();
  img.width = p.width; img.height = p.height;
  img.src = `/page/${p.id}.jpg`;
  $('viewer').querySelector('.count').textContent = `Page ${idx + 1} of ${pages.length}`;
  $('viewer').querySelector('[data-v="prev"]').disabled = idx === 0;
  $('viewer').querySelector('[data-v="next"]').disabled = idx === pages.length - 1;
  fitView(false);
  if (from) { // grow out of the thumbnail that was clicked
    const b = viewerBounds(), r = from.getBoundingClientRect();
    const target = {x: view.x, y: view.y, s: view.s};
    view.s = r.width / p.width;
    view.x = r.left - b.left; view.y = r.top - b.top;
    applyView(false);
    img.getBoundingClientRect(); // make the start position count
    Object.assign(view, target);
    applyView(true);
  }
}

function openViewer(id, thumb) {
  const idx = pages.findIndex((p) => p.id === id);
  if (idx < 0) return;
  const el = $('viewer');
  el.innerHTML = `
    <div class="stage"><img alt="" draggable="false"></div>
    <div class="bar left"><span class="count"></span></div>
    <div class="bar mid">
      <button data-v="out" title="Zoom out (−)" aria-label="Zoom out">${VIEW_ICON.minus}</button>
      <span class="pct">100%</span>
      <button data-v="in" title="Zoom in (+)" aria-label="Zoom in">${VIEW_ICON.plus}</button>
      <button data-v="fit" title="Fit to window (0)" aria-label="Fit to window">${VIEW_ICON.fit}</button>
    </div>
    <div class="bar right"><button data-v="close" title="Close (Esc)" aria-label="Close">${VIEW_ICON.close}</button></div>
    <button class="nav prev" data-v="prev" title="Previous page (←)" aria-label="Previous page">${VIEW_ICON.left}</button>
    <button class="nav next" data-v="next" title="Next page (→)" aria-label="Next page">${VIEW_ICON.right}</button>
    <div class="hint">Scroll to zoom · drag to move · double-click to fit or zoom in</div>`;
  el.hidden = false;
  el.classList.remove('closing');
  view = {idx, s: 1, x: 0, y: 0, fit: 1, nw: 1, nh: 1, drag: null};
  requestAnimationFrame(() => el.classList.add('open'));
  showPage(idx, thumb);
  clearTimeout(view.hintTimer);
  view.hintTimer = setTimeout(() => el.classList.add('hint-gone'), 3500);
  el.classList.remove('hint-gone');
}

function closeViewer() {
  if (!view) return;
  const el = $('viewer');
  view = null;
  el.classList.remove('open');
  setTimeout(() => { if (!view) { el.hidden = true; el.innerHTML = ''; } }, 180);
}

function flip2(delta) {
  const i = view.idx + delta;
  if (i >= 0 && i < pages.length) showPage(i);
}

$('viewer').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-v]');
  if (!b || !view) return;
  const a = b.dataset.v;
  if (a === 'close') closeViewer();
  else if (a === 'in') zoomBy(1.4, null, null, true);
  else if (a === 'out') zoomBy(1 / 1.4, null, null, true);
  else if (a === 'fit') fitView(true);
  else if (a === 'prev') flip2(-1);
  else if (a === 'next') flip2(1);
});

// Wheel: zoom around the pointer. A sideways scroll (or shift) moves the page, as does a pinch's ctrl-wheel on
// systems that report pinches that way, which zooms instead.
$('viewer').addEventListener('wheel', (ev) => {
  if (!view) return;
  ev.preventDefault();
  const b = viewerBounds();
  const px = ev.clientX - b.left, py = ev.clientY - b.top;
  if (ev.ctrlKey) { zoomBy(Math.exp(-ev.deltaY * 0.012), px, py); return; } // pinch
  if (ev.deltaX && Math.abs(ev.deltaX) > Math.abs(ev.deltaY)) { // two-finger sideways scroll
    view.x -= ev.deltaX; constrain(); applyView(false); return;
  }
  zoomBy(Math.exp(-ev.deltaY * (ev.deltaMode === 1 ? 0.05 : 0.0016)), px, py);
}, {passive: false});

// Pinch on the macOS trackpad arrives as gesture events in WebKit.
let pinch = null;
$('viewer').addEventListener('gesturestart', (ev) => { ev.preventDefault(); if (view) pinch = view.s; });
$('viewer').addEventListener('gesturechange', (ev) => {
  ev.preventDefault();
  if (!view || pinch === null) return;
  const b = viewerBounds();
  zoomBy((pinch * ev.scale) / view.s, ev.clientX - b.left, ev.clientY - b.top);
});
$('viewer').addEventListener('gestureend', (ev) => { ev.preventDefault(); pinch = null; });

// Drag to move; a click on the empty background closes the viewer.
$('viewer').addEventListener('pointerdown', (ev) => {
  if (!view || ev.button !== 0 || ev.target.closest('button, .bar')) return;
  view.drag = {x: ev.clientX, y: ev.clientY, vx: view.x, vy: view.y, moved: false, onImage: ev.target.tagName === 'IMG'};
  $('viewer').setPointerCapture(ev.pointerId);
});
$('viewer').addEventListener('pointermove', (ev) => {
  const d = view?.drag;
  if (!d) return;
  const dx = ev.clientX - d.x, dy = ev.clientY - d.y;
  if (!d.moved && Math.hypot(dx, dy) < 4) return;
  d.moved = true;
  $('viewer').classList.add('panning');
  view.x = d.vx + dx; view.y = d.vy + dy;
  constrain(); applyView(false);
});
$('viewer').addEventListener('pointerup', (ev) => {
  const d = view?.drag;
  if (!d) return;
  view.drag = null;
  $('viewer').classList.remove('panning');
  if (!d.moved && !d.onImage) closeViewer();
});
$('viewer').addEventListener('dblclick', (ev) => {
  if (!view || ev.target.closest('button, .bar')) return;
  const b = viewerBounds();
  if (view.s > view.fit * 1.05) fitView(true);
  else zoomBy(Math.min(maxScale(), Math.max(2.5 * view.fit, 1)) / view.s, ev.clientX - b.left, ev.clientY - b.top, true);
});

window.addEventListener('resize', () => { if (view) { const was = view.s / view.fit; fitView(false); zoomBy(was, null, null); } });
document.addEventListener('keydown', (ev) => {
  if (!view) return;
  if (ev.key === 'Escape') { closeViewer(); ev.stopImmediatePropagation(); }
  else if (ev.key === 'ArrowLeft') flip2(-1);
  else if (ev.key === 'ArrowRight') flip2(1);
  else if (ev.key === '+' || ev.key === '=') zoomBy(1.4, null, null, true);
  else if (ev.key === '-') zoomBy(1 / 1.4, null, null, true);
  else if (ev.key === '0') fitView(true);
}, true);
// the page being viewed was deleted or the list changed under it: close rather than show something stale
EventsOn('pages', (list) => { if (view && !(list || []).some((p) => p.id === pages[view.idx]?.id)) closeViewer(); });
