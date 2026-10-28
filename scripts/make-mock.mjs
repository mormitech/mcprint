// Draws the made-up documents of demo mode (mock/pages/*.jpg): six A4 pages at 150 dpi, all fictional.
//
//   node scripts/make-mock.mjs
//
// Needs librsvg (rsvg-convert) and, for the JPEG step, macOS's sips. The pages are committed, so this only has to run
// when the documents change.
import { execFileSync } from 'node:child_process';
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';

const out = resolve(import.meta.dirname, '..', 'mock', 'pages');
const tmp = join(tmpdir(), 'mormiprint-mock');
mkdirSync(out, { recursive: true });
mkdirSync(tmp, { recursive: true });

const W = 1240, H = 1754;
const sans = "Helvetica Neue, Helvetica, Arial, sans-serif";
const hand = "Bradley Hand, Noteworthy, Segoe Script, cursive";
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');
const text = (x, y, s, { size = 26, weight = 400, fill = '#1d232b', family = sans, anchor = 'start', spacing = 0 } = {}) =>
  `<text x="${x}" y="${y}" font-family="${family}" font-size="${size}" font-weight="${weight}" fill="${fill}" text-anchor="${anchor}" letter-spacing="${spacing}">${esc(s)}</text>`;
const line = (x1, y1, x2, y2, stroke = '#c9ced6', w = 2) => `<line x1="${x1}" y1="${y1}" x2="${x2}" y2="${y2}" stroke="${stroke}" stroke-width="${w}"/>`;
const rect = (x, y, w, h, fill, extra = '') => `<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${fill}" ${extra}/>`;
const paragraph = (x, y, lines, opts = {}) => lines.map((l, i) => text(x, y + i * (opts.lead ?? 40), l, opts)).join('');

// A faint grey edge and speckle, as a flatbed leaves them.
const paper = (body) => `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <defs><filter id="n"><feTurbulence type="fractalNoise" baseFrequency="0.9" numOctaves="2" seed="4"/><feColorMatrix values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 .025 0"/></filter></defs>
  ${rect(0, 0, W, H, '#fdfdfc')}${body}
  <rect width="${W}" height="${H}" filter="url(#n)"/>
  ${rect(0, 0, W, 5, '#e9ebee')}${rect(0, H - 5, W, 5, '#e9ebee')}
</svg>`;

const pages = {};

// 1 · invoice
pages['1-invoice'] = paper(`
  ${rect(90, 90, 70, 70, '#2f6f5e')}${text(125, 140, 'A', { size: 54, weight: 700, fill: '#fff', anchor: 'middle' })}
  ${text(180, 118, 'ALDER & FINCH LTD', { size: 34, weight: 700, spacing: 2 })}
  ${text(180, 152, 'Office supplies · 14 Mill Lane · Norwich NR2 4BT', { size: 22, fill: '#59616d' })}
  ${text(1150, 128, 'INVOICE', { size: 58, weight: 300, anchor: 'end', spacing: 6, fill: '#2f6f5e' })}
  ${line(90, 200, 1150, 200, '#2f6f5e', 3)}
  ${text(90, 262, 'BILL TO', { size: 20, weight: 700, fill: '#59616d', spacing: 2 })}
  ${paragraph(90, 300, ['Marlowe Studio', 'Attn. Accounts Payable', '8 Quay Street', 'Bristol BS1 4DE'], { size: 27, lead: 38 })}
  ${paragraph(780, 262, ['Invoice no.', 'Date', 'Due', 'Customer ref.'], { size: 24, fill: '#59616d', lead: 38 })}
  ${paragraph(1150, 262, ['2026-0417', '3 October 2026', '2 November 2026', 'MS-2291'], { size: 24, lead: 38, anchor: 'end' })}
  ${rect(90, 500, 1060, 56, '#eef3f1')}
  ${text(110, 537, 'Description', { size: 22, weight: 700 })}${text(740, 537, 'Qty', { size: 22, weight: 700, anchor: 'end' })}${text(930, 537, 'Unit', { size: 22, weight: 700, anchor: 'end' })}${text(1130, 537, 'Amount', { size: 22, weight: 700, anchor: 'end' })}
  ${[['A4 paper, 80 g, 5 × 500 sheets', 6, 19.5], ['Lever arch files, grey', 24, 3.2], ['Gel pens, black (box of 12)', 10, 7.9], ['Archive boxes, A4', 15, 4.75], ['Delivery', 1, 12]]
    .map(([d, q, u], i) => text(110, 600 + i * 62, d, { size: 26 }) + text(740, 600 + i * 62, String(q), { size: 26, anchor: 'end' }) + text(930, 600 + i * 62, u.toFixed(2), { size: 26, anchor: 'end' }) + text(1130, 600 + i * 62, (q * u).toFixed(2), { size: 26, anchor: 'end' }) + line(90, 624 + i * 62, 1150, 624 + i * 62, '#e3e6ea')).join('')}
  ${paragraph(930, 960, ['Subtotal', 'VAT 20%', 'Total due'], { size: 26, anchor: 'end', lead: 46 })}
  ${paragraph(1130, 960, ['£ 372.50', '£ 74.50', '£ 447.00'], { size: 26, anchor: 'end', lead: 46 })}
  ${line(700, 1018, 1150, 1018, '#1d232b', 2)}
  ${text(90, 1130, 'Payment', { size: 22, weight: 700, fill: '#59616d', spacing: 2 })}
  ${paragraph(90, 1170, ['Bank transfer within 30 days, quoting the invoice number.', 'Account: Alder & Finch Ltd · Sort code 20-45-17 · 60918342'], { size: 24, lead: 38 })}
  ${text(620, 1660, 'Thank you for your order.', { size: 24, anchor: 'middle', fill: '#59616d' })}`);

// 2 · letter
pages['2-letter'] = paper(`
  ${text(90, 140, 'Brightwater Council', { size: 38, weight: 700 })}
  ${paragraph(90, 180, ['Planning & Licensing', 'Civic Offices, 2 Harbour Road', 'Brightwater BW1 3QA'], { size: 23, fill: '#59616d', lead: 32 })}
  ${paragraph(1150, 140, ['Our ref: PL/2026/1183', '5 October 2026'], { size: 23, anchor: 'end', lead: 34 })}
  ${paragraph(90, 380, ['Ms E. Okafor', '41 Linden Close', 'Brightwater BW4 8TT'], { size: 27, lead: 38 })}
  ${text(90, 560, 'Your application: change of use, 41 Linden Close', { size: 28, weight: 700 })}
  ${paragraph(90, 625, [
    'Dear Ms Okafor,',
    '',
    'Thank you for your application of 12 September, which we received on 14 September.',
    'We have now completed our review of the plans and the supporting statement. The',
    'proposed use is acceptable in principle, subject to the three conditions listed on the',
    'enclosed decision notice.',
    '',
    'The first condition concerns the hours of operation, which must end by 18:00 on',
    'weekdays. The second asks for a short noise assessment before the work starts, and',
    'the third for a parking plan for visitors. None of these should delay you if you send',
    'the documents in the next four weeks.',
    '',
    'If you would like to talk it through, please call the number above and ask for the',
    'case officer. You can also reply by post; please quote our reference.',
    '',
    'Yours sincerely,'], { size: 26, lead: 42 })}
  ${rect(90, 1330, 260, 3, '#1d232b')}
  <path d="M96 1300 c 30 -60 60 -70 70 -20 c 10 50 40 40 60 -30 c 14 -50 40 -40 30 10 c -8 40 30 30 70 -20" fill="none" stroke="#25408f" stroke-width="4" stroke-linecap="round"/>
  ${paragraph(90, 1380, ['Daniel Reyes', 'Senior Planning Officer'], { size: 26, lead: 38 })}
  ${text(90, 1500, 'Enc. Decision notice (3 pages)', { size: 22, fill: '#59616d' })}`);

// 3 · handwritten notes
const ruled = Array.from({ length: 29 }, (_, i) => line(70, 260 + i * 50, 1170, 260 + i * 50, '#cfd9ea', 2)).join('');
pages['3-notes'] = paper(`
  ${rect(0, 0, W, H, '#fffef6')}${ruled}${line(150, 100, 150, 1700, '#e7a3a3', 3)}
  ${text(190, 210, 'Team sync — Thursday', { size: 56, family: hand, fill: '#1f3a8a' })}
  ${[['☑ Scanner for the front desk: ordered', 0], ['☑ Move invoices to the shared folder', 1], ['☐ Ask Ines about the archive boxes', 2], ['☐ Retention: keep scans 7 years?', 3]]
    .map(([s, i]) => text(190, 312 + i * 50, s, { size: 38, family: hand, fill: '#1f3a8a' })).join('')}
  ${text(190, 560, 'Scanning', { size: 44, family: hand, fill: '#9a2a2a' })}
  ${['- 300 dpi is plenty for text', '- colour only for photos & stamps', '- both sides when it is a contract', '- name files: date_client_topic'].map((s, i) => text(210, 612 + i * 50, s, { size: 36, family: hand, fill: '#1f3a8a' })).join('')}
  ${text(190, 860, 'Next week', { size: 44, family: hand, fill: '#9a2a2a' })}
  ${['Mon  inventory count', 'Wed  call with the auditor', 'Fri  backup check  ✓'].map((s, i) => text(210, 912 + i * 50, s, { size: 36, family: hand, fill: '#1f3a8a' })).join('')}
  <path d="M190 1130 q 120 -26 260 0 t 260 0" fill="none" stroke="#9a2a2a" stroke-width="3"/>
  ${text(190, 1190, 'Thanks all! — M.', { size: 40, family: hand, fill: '#1f3a8a' })}`);

// 4 · expense form
pages['4-expenses'] = paper(`
  ${rect(0, 0, W, 190, '#25408f')}
  ${text(90, 118, 'EXPENSE CLAIM', { size: 52, weight: 700, fill: '#fff', spacing: 4 })}
  ${text(1150, 118, 'Form EX-2', { size: 28, fill: '#cdd7f4', anchor: 'end' })}
  ${[['Name', 'Priya Nair', 90], ['Department', 'Operations', 90], ['Period', 'September 2026', 90]].map(([k, v], i) => text(90, 270 + i * 70, k, { size: 22, fill: '#59616d' }) + text(330, 270 + i * 70, v, { size: 28 }) + line(330, 282 + i * 70, 820, 282 + i * 70)).join('')}
  ${rect(90, 520, 1060, 54, '#e8ecf8')}
  ${['Date', 'Description', 'Category', 'Amount'].map((h, i) => text([110, 270, 740, 1130][i], 556, h, { size: 22, weight: 700, anchor: i === 3 ? 'end' : 'start' })).join('')}
  ${[['02 Sep', 'Train to Leeds, return', 'Travel', '84.20'], ['02 Sep', 'Lunch with client', 'Meals', '23.60'], ['09 Sep', 'Printer toner', 'Supplies', '61.95'], ['15 Sep', 'Conference ticket', 'Training', '240.00'], ['22 Sep', 'Taxi, airport', 'Travel', '38.40'], ['28 Sep', 'Postage', 'Supplies', '14.30']]
    .map(([d, t, c, a], i) => text(110, 618 + i * 62, d, { size: 25 }) + text(270, 618 + i * 62, t, { size: 25 }) + text(740, 618 + i * 62, c, { size: 25 }) + text(1130, 618 + i * 62, a, { size: 25, anchor: 'end' }) + line(90, 642 + i * 62, 1150, 642 + i * 62, '#e3e6ea')).join('')}
  ${text(930, 1070, 'Total', { size: 28, weight: 700, anchor: 'end' })}${text(1130, 1070, '£ 462.45', { size: 28, weight: 700, anchor: 'end' })}
  ${text(90, 1230, 'Receipts attached: 6', { size: 24, fill: '#59616d' })}
  ${text(90, 1380, 'Signature', { size: 22, fill: '#59616d' })}${line(220, 1388, 620, 1388, '#1d232b')}
  <path d="M232 1366 c 24 -40 40 -34 36 4 c -4 30 26 22 36 -14 c 8 -26 22 -20 18 6 c 22 -34 36 -22 70 6" fill="none" stroke="#25408f" stroke-width="4" stroke-linecap="round"/>
  ${text(700, 1380, 'Approved', { size: 22, fill: '#59616d' })}${line(840, 1388, 1150, 1388, '#1d232b')}`);

// 5 · report with a chart
const bars = [38, 52, 47, 66, 74, 91];
pages['5-report'] = paper(`
  ${text(90, 150, 'Quarterly report', { size: 60, weight: 300, fill: '#2f6f5e' })}
  ${text(90, 200, 'Operations · Q3 2026', { size: 28, fill: '#59616d' })}
  ${line(90, 232, 1150, 232, '#2f6f5e', 3)}
  ${paragraph(90, 310, [
    'Document throughput rose in every month of the quarter. The move to scanning at',
    'the point of receipt removed the weekly backlog, and the average time from arrival',
    'to filing fell from four days to under one.'], { size: 26, lead: 42 })}
  ${text(90, 500, 'Pages scanned per month (thousands)', { size: 24, weight: 700 })}
  ${bars.map((b, i) => rect(130 + i * 170, 1020 - b * 5.2, 110, b * 5.2, i === 5 ? '#2f6f5e' : '#9cc4b8') + text(185 + i * 170, 1020 - b * 5.2 - 12, String(b), { size: 24, anchor: 'middle' }) + text(185 + i * 170, 1062, ['Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep'][i], { size: 24, anchor: 'middle', fill: '#59616d' })).join('')}
  ${line(110, 1020, 1150, 1020, '#1d232b', 2)}
  ${text(90, 1190, 'Highlights', { size: 28, weight: 700 })}
  ${paragraph(110, 1245, ['•  Scan quality complaints down 60% after the move to 300 dpi', '•  Two flatbeds retired; one network scanner now serves the floor', '•  Archive room space freed: 18 shelf metres'], { size: 26, lead: 46 })}
  ${text(620, 1690, '— 3 —', { size: 22, anchor: 'middle', fill: '#59616d' })}`);

// 6 · agreement with a signature
pages['6-agreement'] = paper(`
  ${text(620, 170, 'SERVICE AGREEMENT', { size: 50, weight: 700, anchor: 'middle', spacing: 5 })}
  ${text(620, 220, 'No. SA-2026-088', { size: 24, anchor: 'middle', fill: '#59616d' })}
  ${line(90, 260, 1150, 260, '#1d232b', 2)}
  ${paragraph(90, 330, [
    'This agreement is made on 1 October 2026 between Alder & Finch Ltd (the Supplier)',
    'and Marlowe Studio (the Client).'], { size: 26, lead: 42 })}
  ${[['1.  Services', ['The Supplier will deliver office supplies to the Client\'s premises each month', 'against the standing order attached as Schedule A.']],
     ['2.  Payment', ['Invoices are payable within 30 days. Late payments carry interest at 2% a', 'month on the amount overdue.']],
     ['3.  Term', ['The agreement runs for twelve months and renews for a further twelve unless', 'either party gives sixty days\' notice in writing.']],
     ['4.  Confidentiality', ['Each party keeps the other\'s business information private and uses it only', 'for the purposes of this agreement.']]]
    .map(([h, ls], i) => text(90, 470 + i * 200, h, { size: 28, weight: 700 }) + paragraph(90, 515 + i * 200, ls, { size: 25, lead: 38 })).join('')}
  ${text(90, 1380, 'For the Supplier', { size: 22, fill: '#59616d' })}${text(700, 1380, 'For the Client', { size: 22, fill: '#59616d' })}
  ${line(90, 1500, 540, 1500, '#1d232b')}${line(700, 1500, 1150, 1500, '#1d232b')}
  <path d="M110 1470 c 40 -90 70 -80 60 -10 c -6 40 34 30 60 -34 c 16 -40 40 -20 30 14 c 30 -50 70 -40 110 -4" fill="none" stroke="#25408f" stroke-width="4" stroke-linecap="round"/>
  <path d="M720 1478 c 30 -40 60 -60 80 -20 c 12 28 40 -10 60 -40 c 8 -12 22 4 14 26 c 40 -34 70 -20 110 8" fill="none" stroke="#25408f" stroke-width="4" stroke-linecap="round"/>
  ${text(90, 1536, 'Daniel Reyes, Director', { size: 22 })}${text(700, 1536, 'Priya Nair, Studio Manager', { size: 22 })}`);

for (const [name, svg] of Object.entries(pages)) {
  const svgFile = join(tmp, `${name}.svg`), png = join(tmp, `${name}.png`);
  writeFileSync(svgFile, svg);
  execFileSync('rsvg-convert', ['-w', String(W), '-h', String(H), svgFile, '-o', png]);
  execFileSync('sips', ['-s', 'format', 'jpeg', '-s', 'formatOptions', '74', png, '--out', join(out, `${name}.jpg`)], { stdio: 'ignore' });
}
rmSync(tmp, { recursive: true, force: true });
console.log(`wrote ${Object.keys(pages).length} pages to ${out}`);
