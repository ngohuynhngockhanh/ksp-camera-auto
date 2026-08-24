// Adversarial stress test script for redbida.js logic
const assert = require('assert');

function removeVietnameseTones(str) {
  if (!str || typeof str !== 'string') return '';
  return str
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[đĐ]/g, m => (m === 'đ' ? 'd' : 'D'));
}

console.log('--- 1. Testing Vietnamese diacritic removal ---');
assert.strictEqual(removeVietnameseTones(''), '');
assert.strictEqual(removeVietnameseTones(null), '');
assert.strictEqual(removeVietnameseTones(undefined), '');
assert.strictEqual(removeVietnameseTones(123), '');
assert.strictEqual(removeVietnameseTones('CX King Luxury'), 'CX King Luxury');
assert.strictEqual(removeVietnameseTones('Bida Hoàng Gia Đỉnh Cao 🎱'), 'Bida Hoang Gia Dinh Cao 🎱');
assert.strictEqual(removeVietnameseTones('ĐẮK LẮK ĐỒNG NAI'), 'DAK LAK DONG NAI');
console.log('✓ Vietnamese diacritic removal passed.');

console.log('--- 2. Testing Hashtag sanitization & edge cases ---');
function makeHashtags(title) {
  const cleanTitle = removeVietnameseTones(title).replace(/[^a-zA-Z0-9]/g, '');
  return cleanTitle
    ? `#${cleanTitle} #BILLIARDSlive #INUTlive #highlightsports`
    : '#BILLIARDSlive #INUTlive #highlightsports';
}

assert.strictEqual(makeHashtags(''), '#BILLIARDSlive #INUTlive #highlightsports');
assert.strictEqual(makeHashtags('   '), '#BILLIARDSlive #INUTlive #highlightsports');
assert.strictEqual(makeHashtags('🎱🎱🎱 !!!'), '#BILLIARDSlive #INUTlive #highlightsports');
assert.strictEqual(makeHashtags('CLB Bida 3 Băng Sài Gòn'), '#CLBBida3BangSaiGon #BILLIARDSlive #INUTlive #highlightsports');
assert.strictEqual(makeHashtags('Billiards & Bar (VIP-100%)'), '#BilliardsBarVIP100 #BILLIARDSlive #INUTlive #highlightsports');
console.log('✓ Hashtag sanitization passed.');

console.log('--- 3. Testing 20-tab INI generation ---');
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

const ini = generateIni('Bida King');
const sections = ini.split('\n\n');
assert.strictEqual(sections.length, 20);
assert.ok(sections[0].startsWith('[C01]'));
assert.ok(sections[19].startsWith('[C20]'));
assert.ok(sections[0].includes('vid_play_label=Bida King'));
assert.ok(sections[19].includes('vid_play_label=Bida King'));
console.log('✓ 20-tab INI generation passed.');

console.log('--- 4. Testing CSS gradient sanitization ---');
function sanitizeBg(rawBg) {
  const bgInputVal = (rawBg || 'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )').trim();
  return bgInputVal.replace(/;\s*$/, '').trim();
}

assert.strictEqual(sanitizeBg('linear-gradient(#fff, #000);'), 'linear-gradient(#fff, #000)');
assert.strictEqual(sanitizeBg('linear-gradient(#fff, #000) ;  '), 'linear-gradient(#fff, #000)');
assert.strictEqual(sanitizeBg('linear-gradient(#fff, #000)'), 'linear-gradient(#fff, #000)');
assert.strictEqual(sanitizeBg(''), 'radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )');
console.log('✓ CSS gradient sanitization passed.');

console.log('--- 5. Testing Group alias matching (redbidaMatchGroup) ---');
function matchGroup(target, options) {
  if (!target) return '';
  if (options.includes(target)) return target;
  const norm = s => s.toLowerCase().replace(/&/g, '/').replace(/\s+/g, '');
  const targetNorm = norm(target);
  for (const opt of options) {
    if (opt && norm(opt) === targetNorm) return opt;
  }
  if (targetNorm.includes('stream') || targetNorm.includes('video')) {
    const found = options.find(o => norm(o).includes('stream') || norm(o).includes('video') || norm(o) === 'livestream');
    if (found) return found;
  }
  if (targetNorm.includes('sched') || targetNorm.includes('maint')) {
    const found = options.find(o => norm(o).includes('sched') || norm(o).includes('maint'));
    if (found) return found;
  }
  if (targetNorm.includes('brand') || targetNorm.includes('logo')) {
    const found = options.find(o => norm(o).includes('brand') || norm(o).includes('logo'));
    if (found) return found;
  }
  if (targetNorm.includes('sec') || targetNorm.includes('cred') || targetNorm.includes('shinobi')) {
    const found = options.find(o => norm(o).includes('sec') || norm(o).includes('cred'));
    if (found) return found;
  }
  return '';
}

const mockGroups = ['Branding / Logo', 'Video & Streaming', 'Security / Credentials', 'Schedule / Maintenance', 'UI / Display'];
assert.strictEqual(matchGroup('Branding / Logo', mockGroups), 'Branding / Logo');
assert.strictEqual(matchGroup('Video & Streaming', mockGroups), 'Video & Streaming');
assert.strictEqual(matchGroup('Streaming', mockGroups), 'Video & Streaming');
assert.strictEqual(matchGroup('Shinobi', mockGroups), 'Security / Credentials');
assert.strictEqual(matchGroup('Schedule & Maintenance', mockGroups), 'Schedule / Maintenance');
console.log('✓ Group alias matching passed.');

console.log('ALL ADVERSARIAL UNIT STRESS TESTS PASSED!');
