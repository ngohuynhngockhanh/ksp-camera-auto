/**
 * Hermetic Verification Script for Milestone 3 (RedBida Knowledge Hub & Preset Generator)
 * Challenger 1 - Milestone 3
 */

const fs = require('fs');
const path = require('path');
const vm = require('vm');

let passedTests = 0;
let failedTests = 0;

function assert(condition, message) {
  if (!condition) {
    console.error(`❌ FAIL: ${message}`);
    failedTests++;
    throw new Error(`Assertion failed: ${message}`);
  } else {
    console.log(`✅ PASS: ${message}`);
    passedTests++;
  }
}

function assertEqual(actual, expected, message) {
  if (actual !== expected) {
    console.error(`❌ FAIL: ${message} (Expected: ${JSON.stringify(expected)}, Got: ${JSON.stringify(actual)})`);
    failedTests++;
    throw new Error(`Assertion failed: ${message}`);
  } else {
    console.log(`✅ PASS: ${message}`);
    passedTests++;
  }
}

// 1. Read redbida.js code
const redbidaJsPath = path.resolve(__dirname, '../../web/static/redbida.js');
const redbidaCode = fs.readFileSync(redbidaJsPath, 'utf8');

// 2. Setup mock DOM environment
function createMockDOM() {
  const elements = new Map();
  const listeners = new Map();

  function makeElement(id, tagName = 'div', initialProps = {}) {
    const el = {
      id,
      tagName: tagName.toUpperCase(),
      value: initialProps.value || '',
      textContent: initialProps.textContent || '',
      innerHTML: initialProps.innerHTML || '',
      className: initialProps.className || '',
      style: { display: '', background: '' },
      dataset: { ...initialProps.dataset },
      classList: {
        classes: new Set(initialProps.classes || []),
        add(c) { this.classes.add(c); },
        remove(c) { this.classes.delete(c); },
        toggle(c, force) {
          if (force !== undefined) {
            if (force) this.classes.add(c);
            else this.classes.delete(c);
            return force;
          }
          if (this.classes.has(c)) { this.classes.delete(c); return false; }
          this.classes.add(c); return true;
        },
        contains(c) { return this.classes.has(c); },
      },
      options: initialProps.options || [],
      querySelector(sel) {
        if (sel === '.redbida-preview-title') {
          return elements.get('mock-preview-title') || makeElement('mock-preview-title', 'span', { textContent: 'CX King Luxury' });
        }
        if (sel === '[data-preview-key="ui_bg"]') {
          return elements.get('mock-row-bg-preview') || makeElement('mock-row-bg-preview', 'div');
        }
        return null;
      },
      querySelectorAll(sel) {
        if (sel.includes('.redbida-swatch')) {
          return Array.from(elements.values()).filter(e => e.dataset && e.dataset.bg);
        }
        return [];
      },
      closest(sel) {
        if (this.dataset && this.dataset.bg) return this;
        return null;
      },
      addEventListener(event, fn) {
        if (!listeners.has(id + ':' + event)) listeners.set(id + ':' + event, []);
        listeners.get(id + ':' + event).push(fn);
      },
      scrollIntoView() {},
    };
    elements.set(id, el);
    return el;
  }

  // Pre-populate known elements
  makeElement('redbida-msg', 'div');
  makeElement('redbida-preset-title', 'input', { value: 'CX King Luxury' });
  makeElement('redbida-preset-count', 'input', { value: '8' });
  makeElement('redbida-preset-groupkey', 'input', { value: 'CX_KING_LUXURY' });
  makeElement('redbida-preset-ggcode', 'input', { value: 'G-SFSDZPR95Z' });
  makeElement('redbida-preset-bg', 'input', { value: 'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )' });
  makeElement('redbida-preset-bg-preview', 'div');
  makeElement('redbida-preset-diff', 'div');
  makeElement('redbida-draft-count', 'div');
  makeElement('redbida-key-count', 'div');
  makeElement('redbida-broker-status', 'div');
  makeElement('redbida-group', 'select', {
    options: [
      { value: '' },
      { value: 'Branding / Logo' },
      { value: 'Video & Streaming' },
      { value: 'Security / Credentials' },
      { value: 'Schedule & Maintenance' },
    ]
  });
  makeElement('redbida-tbody', 'tbody');

  // Swatches
  const swatches = [
    'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )',
    'linear-gradient(135deg, #093028 0%, #237a57 100%)',
    'linear-gradient(135deg, #0f2027 0%, #203a43 50%, #2c5364 100%)',
    'linear-gradient(135deg, #4b1248 0%, #f0c27b 100%)',
    'linear-gradient(135deg, #20002c 0%, #cbb4d4 100%)',
    'linear-gradient(135deg, #1f1c2c 0%, #928dab 100%)',
  ];
  swatches.forEach((bg, idx) => {
    makeElement(`swatch-${idx}`, 'button', { dataset: { bg }, classes: idx === 0 ? ['redbida-swatch', 'active'] : ['redbida-swatch'] });
  });

  const doc = {
    getElementById(id) {
      return elements.get(id) || null;
    },
    querySelector(sel) {
      if (sel === '#redbida-preset-swatches') return elements.get('redbida-preset-swatches') || makeElement('redbida-preset-swatches', 'div');
      if (sel === '[data-preview-key="ui_bg"]') return elements.get('mock-row-bg-preview') || makeElement('mock-row-bg-preview', 'div');
      return null;
    },
    querySelectorAll(sel) {
      if (sel.includes('.redbida-swatch')) {
        return Array.from(elements.values()).filter(e => e.dataset && e.dataset.bg);
      }
      if (sel.includes('.redbida-pillar-btn')) {
        return [];
      }
      return [];
    },
    addEventListener() {},
  };

  return { doc, elements };
}

// 3. Create Sandbox & Execute redbida.js
const { doc, elements } = createMockDOM();

const sandbox = {
  document: doc,
  window: {
    confirm: () => true,
    addEventListener: () => {},
  },
  console: console,
  escapeHtml: (s) => String(s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
  api: async () => ({}),
  FileReader: class {
    readAsDataURL() {}
  },
  getComputedStyle: () => ({ display: 'block' }),
};

vm.createContext(sandbox);
vm.runInContext(redbidaCode, sandbox);

console.log('--- STARTING EMPIRICAL TEST SUITE ---\n');

// ----------------------------------------------------
// TEST SUITE 1: removeVietnameseTones Diacritic Stress Testing
// ----------------------------------------------------
console.log('--- Suite 1: removeVietnameseTones Stress Tests ---');

const toneTests = [
  { input: 'CX King Luxury', expected: 'CX King Luxury' },
  { input: 'Bida Hoàng Gia', expected: 'Bida Hoang Gia' },
  { input: 'Câu Lạc Bộ Billiards Đỉnh Cao', expected: 'Cau Lac Bo Billiards Dinh Cao' },
  { input: 'Đồng Nai & Đắk Lắk & Đà Nẵng', expected: 'Dong Nai & Dak Lak & Da Nang' },
  { input: 'Quán 88 - Bắn Là Trúng!', expected: 'Quan 88 - Ban La Trung!' },
  { input: 'Á À Ả Ã Ạ Ă Ắ Ằ Ẳ Ẵ Ặ Â Ấ Ầ Ẩ Ẫ Ậ', expected: 'A A A A A A A A A A A A A A A A A' },
  { input: 'é è ẻ ẽ ẹ ê ế ề ể ễ ệ', expected: 'e e e e e e e e e e e' },
  { input: 'í ì ỉ ĩ ị', expected: 'i i i i i' },
  { input: 'ó ò ỏ õ ọ ô ố ồ ổ ỗ ộ ơ ớ ờ ở ỡ ợ', expected: 'o o o o o o o o o o o o o o o o o' },
  { input: 'ú ù ủ ũ ụ ư ứ ừ ử ữ ự', expected: 'u u u u u u u u u u u' },
  { input: 'ý ỳ ỷ ỹ ỵ', expected: 'y y y y y' },
  { input: 'đ Đ', expected: 'd D' },
  { input: '', expected: '' },
  { input: null, expected: '' },
  { input: undefined, expected: '' },
  { input: 12345, expected: '' },
];

toneTests.forEach(({ input, expected }, idx) => {
  const actual = sandbox.removeVietnameseTones(input);
  assertEqual(actual, expected, `removeVietnameseTones case ${idx + 1}: ${JSON.stringify(input)}`);
});

// ----------------------------------------------------
// TEST SUITE 2: Hashtag Generator Formatting & Sanitization
// ----------------------------------------------------
console.log('\n--- Suite 2: Hashtag Generator Tests ---');

function generateHashtags(title) {
  const cleanTitle = sandbox.removeVietnameseTones(title).replace(/[^a-zA-Z0-9]/g, '');
  return cleanTitle
    ? `#${cleanTitle} #BILLIARDSlive #INUTlive #highlightsports`
    : '#BILLIARDSlive #INUTlive #highlightsports';
}

const hashtagCases = [
  {
    title: 'CX King Luxury',
    expected: '#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports',
  },
  {
    title: 'SD Billiards Club - CS2',
    expected: '#SDBilliardsClubCS2 #BILLIARDSlive #INUTlive #highlightsports',
  },
  {
    title: 'Bida Hoàng Gia (Cơ sở 1)',
    expected: '#BidaHoangGiaCoso1 #BILLIARDSlive #INUTlive #highlightsports',
  },
  {
    title: 'CLB Bida 3 Băng & Lỗ 24/7!',
    expected: '#CLBBida3BangLo247 #BILLIARDSlive #INUTlive #highlightsports',
  },
  {
    title: '!@#$%^&*()',
    expected: '#BILLIARDSlive #INUTlive #highlightsports',
  },
  {
    title: '',
    expected: '#BILLIARDSlive #INUTlive #highlightsports',
  },
];

hashtagCases.forEach(({ title, expected }, idx) => {
  const actual = generateHashtags(title);
  assertEqual(actual, expected, `Hashtags case ${idx + 1}: "${title}"`);
});

// ----------------------------------------------------
// TEST SUITE 3: 20-Tab INI Generator Verification
// ----------------------------------------------------
console.log('\n--- Suite 3: 20-Tab INI Generator Verification ---');

function generateIni(title) {
  const sections = [];
  for (let i = 1; i <= 20; i++) {
    const pad = String(i).padStart(2, '0');
    sections.push(
      `[C${pad}]\n` +
      `stream_label=Video Trực tiếp\n` +
      `vid_list_label=Danh sách highlight\n` +
      `vid_play_label=${title}\n` +
      `list_refresh_label=Cập nhật highlight`
    );
  }
  return sections.join('\n\n');
}

const iniOutput = generateIni('CX King Luxury');
const iniSections = iniOutput.split('\n\n');

assertEqual(iniSections.length, 20, 'INI output must have exactly 20 sections');

iniSections.forEach((sec, idx) => {
  const num = idx + 1;
  const pad = String(num).padStart(2, '0');
  const lines = sec.split('\n');
  
  assertEqual(lines.length, 5, `Section C${pad} must have 5 lines (header + 4 properties)`);
  assertEqual(lines[0], `[C${pad}]`, `Section header must be [C${pad}]`);
  assertEqual(lines[1], 'stream_label=Video Trực tiếp', `Section C${pad} line 1`);
  assertEqual(lines[2], 'vid_list_label=Danh sách highlight', `Section C${pad} line 2`);
  assertEqual(lines[3], 'vid_play_label=CX King Luxury', `Section C${pad} line 3 (matches ui_title)`);
  assertEqual(lines[4], 'list_refresh_label=Cập nhật highlight', `Section C${pad} line 4`);
});

// ----------------------------------------------------
// TEST SUITE 4: Full redbidaGeneratePreset Integration Test
// ----------------------------------------------------
console.log('\n--- Suite 4: redbidaGeneratePreset Integration ---');

// Set inputs
elements.get('redbida-preset-title').value = 'Bida Hoàng Gia CS1';
elements.get('redbida-preset-count').value = '10';
elements.get('redbida-preset-bg').value = 'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% );   ';

// Execute generator
sandbox.window.redbidaGeneratePreset();

const drafts = sandbox.window.redbidaState.drafts;

assert(drafts.has('ui_title'), 'Drafts must contain ui_title');
assertEqual(drafts.get('ui_title'), 'Bida Hoàng Gia CS1', 'ui_title value');

assert(drafts.has('company_name'), 'Drafts must contain company_name');
assertEqual(drafts.get('company_name'), 'Bida Hoàng Gia CS1', 'company_name value');

assert(drafts.has('camera_count'), 'Drafts must contain camera_count');
assertEqual(drafts.get('camera_count'), 10, 'camera_count value (integer 10)');

assert(drafts.has('toolbar_show_count'), 'Drafts must contain toolbar_show_count');
assertEqual(drafts.get('toolbar_show_count'), 10, 'toolbar_show_count value (matches camera_count)');

assert(drafts.has('ui_bg'), 'Drafts must contain ui_bg');
assertEqual(
  drafts.get('ui_bg'),
  'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )',
  'ui_bg trailing semicolon must be stripped'
);

assert(drafts.has('custom_hashtags'), 'Drafts must contain custom_hashtags');
assertEqual(
  drafts.get('custom_hashtags'),
  '#BidaHoangGiaCS1 #BILLIARDSlive #INUTlive #highlightsports',
  'custom_hashtags sanitized'
);

assert(drafts.has('video_config'), 'Drafts must contain video_config');
assertEqual(drafts.get('video_config'), 'range=72', 'video_config is range=72');

assert(drafts.has('hls_using_go2rtc'), 'Drafts must contain hls_using_go2rtc');
assertEqual(drafts.get('hls_using_go2rtc'), true, 'hls_using_go2rtc is true');

assert(drafts.has('hls_using_go2rtc_livestream'), 'Drafts must contain hls_using_go2rtc_livestream');
assertEqual(drafts.get('hls_using_go2rtc_livestream'), true, 'hls_using_go2rtc_livestream is true');

assert(drafts.has('hls_using_go2rtc_tiktok'), 'Drafts must contain hls_using_go2rtc_tiktok');
assertEqual(drafts.get('hls_using_go2rtc_tiktok'), true, 'hls_using_go2rtc_tiktok is true');

assert(drafts.has('ui_scoreboard'), 'Drafts must contain ui_scoreboard');
assertEqual(drafts.get('ui_scoreboard'), true, 'ui_scoreboard is true');

assert(drafts.has('logo_header'), 'Drafts must contain logo_header');
assertEqual(
  drafts.get('logo_header'),
  'https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png',
  'logo_header golden URL'
);

assert(drafts.has('logo_header_text'), 'Drafts must contain logo_header_text');
assertEqual(
  drafts.get('logo_header_text'),
  'Billiard Live - Tải clip bàn bida và livestream',
  'logo_header_text golden slogan'
);

assert(drafts.has('button_generate_go2rtc_stream'), 'Drafts must contain button_generate_go2rtc_stream');
assertEqual(drafts.get('button_generate_go2rtc_stream'), true, 'button_generate_go2rtc_stream is true');

assert(drafts.has('ui_tabs_links'), 'Drafts must contain ui_tabs_links');
assert(drafts.get('ui_tabs_links').includes('vid_play_label=Bida Hoàng Gia CS1'), 'ui_tabs_links contains correct vid_play_label');
assert(drafts.get('ui_tabs_links').includes('[C01]') && drafts.get('ui_tabs_links').includes('[C20]'), 'ui_tabs_links spans [C01]..[C20]');

// Check Visual Diff Card Rendering
const diffEl = elements.get('redbida-preset-diff');
assert(diffEl.innerHTML.includes('redbida-diff-card'), 'Diff card HTML must contain redbida-diff-card');
assert(diffEl.innerHTML.includes('15 tham số'), 'Diff card must state 15 parameters');
assert(diffEl.innerHTML.includes('ui_title'), 'Diff card table must list ui_title');
assert(diffEl.innerHTML.includes('custom_hashtags'), 'Diff card table must list custom_hashtags');
assertEqual(diffEl.style.display, 'block', 'Diff card display must be set to block');

// ----------------------------------------------------
// TEST SUITE 5: 4-Pillar Group Matching Logic
// ----------------------------------------------------
console.log('\n--- Suite 5: 4-Pillar Group Matching ---');

const matchCases = [
  { input: 'Branding / Logo', expected: 'Branding / Logo' },
  { input: 'Branding', expected: 'Branding / Logo' },
  { input: 'Logo', expected: 'Branding / Logo' },
  { input: 'Video & Streaming', expected: 'Video & Streaming' },
  { input: 'Streaming', expected: 'Video & Streaming' },
  { input: 'Livestream', expected: 'Video & Streaming' },
  { input: 'Security / Credentials', expected: 'Security / Credentials' },
  { input: 'Shinobi', expected: 'Security / Credentials' },
  { input: 'Credentials', expected: 'Security / Credentials' },
  { input: 'Schedule & Maintenance', expected: 'Schedule & Maintenance' },
  { input: 'Maintenance', expected: 'Schedule & Maintenance' },
];

matchCases.forEach(({ input, expected }, idx) => {
  const actual = vm.runInContext(`redbidaMatchGroup(${JSON.stringify(input)})`, sandbox);
  assertEqual(actual, expected, `Group Match case ${idx + 1}: "${input}" -> "${expected}"`);
});

// ----------------------------------------------------
// TEST SUITE 6: Quick Action Go2RTC Stream Trigger
// ----------------------------------------------------
console.log('\n--- Suite 6: Quick Action Trigger Tests ---');

sandbox.window.redbidaTriggerGo2RTCStream();
assertEqual(
  sandbox.window.redbidaState.drafts.get('button_generate_go2rtc_stream'),
  true,
  'redbidaTriggerGo2RTCStream sets button_generate_go2rtc_stream to true'
);

// ----------------------------------------------------
// TEST SUITE 7: Reset Preset Form
// ----------------------------------------------------
console.log('\n--- Suite 7: Reset Preset Form ---');

sandbox.window.redbidaResetPresetForm();
assertEqual(elements.get('redbida-preset-title').value, 'CX King Luxury', 'Reset title');
assertEqual(elements.get('redbida-preset-count').value, '8', 'Reset count');
assertEqual(elements.get('redbida-preset-groupkey').value, 'CX_KING_LUXURY', 'Reset groupkey');
assertEqual(elements.get('redbida-preset-ggcode').value, 'G-SFSDZPR95Z', 'Reset ggcode');
assertEqual(elements.get('redbida-preset-diff').style.display, 'none', 'Reset hides diff card');

console.log('\n========================================');
console.log(`TOTAL TESTS: ${passedTests + failedTests}`);
console.log(`PASSED: ${passedTests}`);
console.log(`FAILED: ${failedTests}`);
console.log('========================================');

if (failedTests > 0) {
  process.exit(1);
} else {
  process.exit(0);
}
