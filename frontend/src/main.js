import './style.css';
import {
  Version, Devices, Search, AddDevice, RemoveDevice, Choices, Scan, CancelScan, Pages, DeletePage, MovePage,
  ClearPages, ExportPDF, LoadPrefs, SavePrefs, Status,
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]));
const SOURCE = {platen: 'Flatbed', feeder: 'Feeder'};
const COLOR = {color: 'Colour', gray: 'Grey', bw: 'Black & white'};
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
      <label>Source<select id="o-source" ${busy ? 'disabled' : ''}>${choices.sources.map((s) => opt(SOURCE, s, opts.source)).join('')}</select></label>
      ${choices.canDuplex && opts.source === 'feeder' ? `<label class="check"><input type="checkbox" id="o-duplex" ${opts.duplex ? 'checked' : ''} ${busy ? 'disabled' : ''}> Both sides</label>` : ''}
      <label>Colour<select id="o-color" ${busy ? 'disabled' : ''}>${choices.colors.map((c) => opt(COLOR, c, opts.color)).join('')}</select></label>
      <label>Resolution<select id="o-dpi" ${busy ? 'disabled' : ''}>${choices.dpis.map((v) => `<option value="${v}" ${v === opts.dpi ? 'selected' : ''}>${v} dpi</option>`).join('')}</select></label>
      <label>Paper<select id="o-paper" ${busy ? 'disabled' : ''}>${choices.papers.map((p) => opt(PAPER, p, opts.paper)).join('')}</select></label>
      ${busy
        ? '<button id="cancel" class="danger">Cancel</button>'
        : `<div class="scanbox"><button id="scan" class="primary" ${d.online ? '' : 'disabled'}>Scan</button>
             <span class="hint">${pages.length ? `adds page ${pages.length + 1} to this document` : 'starts a new document'}</span></div>`}
    </div>`;
  } else if (!scan.message) {
    form = '<p class="empty">Reading what the scanner can do…</p>';
  }
  const hint = !d.online && !busy ? '<div class="state error">This scanner is offline right now.</div>' : '';
  const st = scan.message ? `<div class="state ${scan.state}">${esc(scan.message)}</div>` : '';
  el.innerHTML = `<div class="row"><strong>${esc(d.name)}</strong><span class="muted">${esc(d.host)}</span></div><div style="height:10px"></div>${form}${hint}${st}`;
}

$('controls').addEventListener('change', async (ev) => {
  const t = ev.target;
  if (t.id === 'o-source') { opts.source = t.value; opts.duplex = false; }
  else if (t.id === 'o-duplex') opts.duplex = t.checked;
  else if (t.id === 'o-color') opts.color = t.value;
  else if (t.id === 'o-dpi') opts.dpi = Number(t.value);
  else if (t.id === 'o-paper') opts.paper = t.value;
  else return;
  if (t.id === 'o-source' || t.id === 'o-duplex') {
    try { await loadChoices(); } catch (e) { scan = {state: 'error', message: String(e)}; }
  }
  renderControls();
  savePrefs();
});

$('controls').addEventListener('click', async (ev) => {
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
  renderControls();
  const bar = $('pages-bar');
  bar.innerHTML = pages.length
    ? `<span class="grow">${pages.length} page${pages.length === 1 ? '' : 's'}</span>
       <button id="pdf" class="primary" ${scan.state === 'scanning' ? 'disabled' : ''}>Save as PDF…</button>
       <button id="clear" class="danger" ${scan.state === 'scanning' ? 'disabled' : ''}>Clear all</button>`
    : '';
  $('pages').innerHTML = pages.map((p, i) => `
    <div class="page" data-id="${p.id}">
      <img src="/page/${p.id}.jpg" alt="Page ${i + 1}" data-view="${p.id}" loading="lazy">
      <div class="tools">
        <span>${i + 1} · ${p.width}×${p.height}</span>
        <button data-move="-1" ${i === 0 ? 'disabled' : ''} title="Move earlier">←</button>
        <button data-move="1" ${i === pages.length - 1 ? 'disabled' : ''} title="Move later">→</button>
        <button data-del="1" class="danger" title="Delete page">✕</button>
      </div>
    </div>`).join('');
}

$('pages').addEventListener('click', async (ev) => {
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
  $('ver').textContent = `mcprint v${await Version()}`;
  try { prefs = {last: '', devices: {}, ...JSON.parse(await LoadPrefs())}; } catch {}
  pages = (await Pages()) || [];
  renderPages();
  renderControls();
  await setDevices(await Devices());
})();
