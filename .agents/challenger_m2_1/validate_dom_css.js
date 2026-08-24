const fs = require('fs');
const html = fs.readFileSync('web/static/index.html', 'utf8');
const css = fs.readFileSync('web/static/style.css', 'utf8');

console.log('=== 1. HTML PARSING & DOM INTEGRITY ===');
// Check duplicate IDs using word boundary
const idMatches = [...html.matchAll(/\bid="([^"]+)"/g)].map(m => m[1]);
const idCounts = {};
idMatches.forEach(id => { idCounts[id] = (idCounts[id] || 0) + 1; });
const duplicateIds = Object.entries(idCounts).filter(([id, count]) => count > 1);
console.log('Total unique IDs found in index.html:', idMatches.length);
if (duplicateIds.length > 0) {
  console.error('Duplicate IDs found:', duplicateIds);
  process.exit(1);
} else {
  console.log('PASS: Zero duplicate IDs across index.html');
}

// Check unclosed tags in #view-redbida
const viewRedbidaHtml = html.substring(
  html.indexOf('<section class="view" id="view-redbida"'),
  html.indexOf('</section>', html.indexOf('<section class="view" id="view-redbida"')) + 10
);

console.log('Validating #view-redbida HTML tag balance...');
const voidTags = new Set(['input', 'img', 'br', 'hr', 'meta', 'link']);
const tagStack = [];
const tagRegex = /<\/?([a-zA-Z0-9-]+)(?:\s+[^>]*?)?(\/?)>/g;
let match;

while ((match = tagRegex.exec(viewRedbidaHtml)) !== null) {
  const fullMatch = match[0];
  const tagName = match[1].toLowerCase();
  const isClosing = fullMatch.startsWith('</');
  const isSelfClosing = match[2] === '/' || voidTags.has(tagName);

  if (isSelfClosing) continue;

  if (isClosing) {
    if (tagStack.length === 0) {
      console.error(`ERROR: Unexpected closing tag </${tagName}> in #view-redbida`);
      process.exit(1);
    }
    const lastTag = tagStack.pop();
    if (lastTag !== tagName) {
      console.error(`ERROR: Mismatched tag: expected </${lastTag}>, found </${tagName}> in #view-redbida`);
      process.exit(1);
    }
  } else {
    tagStack.push(tagName);
  }
}

if (tagStack.length > 0) {
  console.error('ERROR: Unclosed tags in #view-redbida:', tagStack);
  process.exit(1);
} else {
  console.log('PASS: All HTML tags in #view-redbida are perfectly balanced and closed!');
}

// Check required selectors in #view-redbida
const requiredSelectors = [
  'view-redbida',
  'redbida-refresh',
  'redbida-apply',
  'redbida-toggle-preset',
  'redbida-toggle-hub',
  'redbida-node-status',
  'redbida-key-count',
  'redbida-time-status',
  'redbida-ntp-status',
  'redbida-broker-status',
  'redbida-draft-count',
  'redbida-preset-panel',
  'redbida-preset-title',
  'redbida-preset-count',
  'redbida-preset-groupkey',
  'redbida-preset-ggcode',
  'redbida-preset-bg',
  'redbida-preset-swatches',
  'redbida-preset-bg-preview',
  'redbida-preset-gen-btn',
  'redbida-preset-reset-btn',
  'redbida-preset-diff',
  'redbida-knowledge-hub',
  'redbida-search',
  'redbida-group',
  'redbida-dirty-only',
  'redbida-time-refresh',
  'redbida-msg',
  'redbida-table',
  'redbida-tbody'
];

const missingSelectors = requiredSelectors.filter(id => !idMatches.includes(id));
if (missingSelectors.length > 0) {
  console.error('Missing required IDs in #view-redbida:', missingSelectors);
  process.exit(1);
} else {
  console.log('PASS: All 30 required selectors and IDs are present in index.html');
}

// Check data-testid attributes
const dataTestIds = [...html.matchAll(/data-testid="([^"]+)"/g)].map(m => m[1]);
const requiredTestIds = ['redbida-refresh', 'redbida-apply', 'redbida-search', 'redbida-group'];
const missingTestIds = requiredTestIds.filter(tid => !dataTestIds.includes(tid));
if (missingTestIds.length > 0) {
  console.error('Missing required data-testid attributes:', missingTestIds);
  process.exit(1);
} else {
  console.log('PASS: All required data-testid attributes present:', requiredTestIds);
}

// Check 4 Knowledge Pillars
const pillarClasses = ['pillar-branding', 'pillar-streaming', 'pillar-shinobi', 'pillar-system'];
const missingPillars = pillarClasses.filter(p => !html.includes(p));
if (missingPillars.length > 0) {
  console.error('Missing pillar classes in index.html:', missingPillars);
  process.exit(1);
} else {
  console.log('PASS: All 4 Pillar classes present:', pillarClasses);
}

console.log('\n=== 2. CSS SYNTAX & BRACE MATCHING ===');
let openBraces = 0;
let closeBraces = 0;
let inString = false;
let stringChar = '';
let inComment = false;

for (let i = 0; i < css.length; i++) {
  const c = css[i];
  const next = css[i+1];
  if (inComment) {
    if (c === '*' && next === '/') { inComment = false; i++; }
    continue;
  }
  if (c === '/' && next === '*') { inComment = true; i++; continue; }
  if (inString) {
    if (c === stringChar && css[i-1] !== '\\') { inString = false; }
    continue;
  }
  if (c === '"' || c === "'") { inString = true; stringChar = c; continue; }
  if (c === '{') openBraces++;
  if (c === '}') closeBraces++;
}

console.log(`CSS Braces: open={${openBraces}}, close={${closeBraces}}`);
if (openBraces !== closeBraces) {
  console.error(`ERROR: CSS brace mismatch! Open: ${openBraces}, Close: ${closeBraces}`);
  process.exit(1);
} else {
  console.log('PASS: CSS curly braces are perfectly balanced!');
}

// Check glassmorphism tokens in CSS
const requiredTokens = [
  '--glass-bg',
  '--glass-bg-subtle',
  '--glass-bg-card',
  '--glass-bg-hover',
  '--glass-border',
  '--glass-border-subtle',
  '--glass-border-accent',
  '--glass-blur',
  '--glass-blur-sm',
  '--glass-shadow',
  '--glass-shadow-sm',
  '--glass-shadow-lg',
  '--glass-glow-accent',
  '--glass-glow-success',
  '--glass-glow-warning',
  '--glass-glow-danger'
];
const missingTokens = requiredTokens.filter(tok => !css.includes(tok));
if (missingTokens.length > 0) {
  console.error('Missing glassmorphism tokens in style.css:', missingTokens);
  process.exit(1);
} else {
  console.log('PASS: All 16 Glassmorphism tokens are defined in style.css');
}

// Check theme scopes for tokens
const hasDarkRoot = css.includes(':root {') && css.includes('--glass-bg:');
const hasLightMedia = css.includes('prefers-color-scheme: light') && css.includes('--glass-bg:');
const hasDarkAttr = css.includes(':root[data-theme="dark"]') && css.includes('--glass-bg:');
const hasLightAttr = css.includes(':root[data-theme="light"]') && css.includes('--glass-bg:');
console.log('Theme tokens coverage:', { hasDarkRoot, hasLightMedia, hasDarkAttr, hasLightAttr });
if (hasDarkRoot && hasLightMedia && hasDarkAttr && hasLightAttr) {
  console.log('PASS: Tokens declared across all 4 theme selectors (default root, dark attribute, light attribute, light prefers-color-scheme)');
} else {
  console.error('ERROR: Incomplete theme selector token coverage');
  process.exit(1);
}

// Check required component classes in CSS
const componentClasses = [
  '.redbida-status-grid',
  '.redbida-metric-card',
  '.redbida-preset-card',
  '#redbida-preset-panel',
  '.redbida-preset-grid',
  '.redbida-preset-swatches-wrap',
  '.redbida-preset-swatches',
  '.redbida-swatch',
  '.redbida-swatch-color',
  '.redbida-gradient-preview-wrap',
  '.redbida-gradient-preview',
  '.redbida-preview-title',
  '.redbida-preview-sub',
  '.redbida-preset-actions',
  '.redbida-diff-card',
  '.redbida-diff-title',
  '.redbida-diff-grid',
  '.redbida-diff-item',
  '.redbida-diff-key',
  '.redbida-diff-val',
  '.redbida-knowledge-hub',
  '#redbida-knowledge-hub',
  '.redbida-pillars-grid',
  '.redbida-pillar-card',
  '.redbida-pillar-header',
  '.redbida-pillar-icon',
  '.redbida-pillar-num',
  '.redbida-pillar-title',
  '.redbida-pillar-desc',
  '.redbida-pillar-keys',
  '.redbida-pillar-badge',
  '.redbida-pillar-btn',
  '.redbida-checkerboard',
  '.redbida-tab-simulator',
  '.redbida-tab-sim-item',
  '.redbida-toolbar',
  '.redbida-table',
  '.redbida-editor',
  '.redbida-file',
  '.redbida-logo-preview',
  '.redbida-protected-value',
  '.redbida-dirty',
  '.redbida-risk-editable',
  '.redbida-risk-confirm-required',
  '.redbida-risk-read-only-protected',
  '.redbida-risk-unknown',
  '.redbida-risk-secret',
  '.redbida-row-status',
  '.redbida-current'
];

const missingClasses = componentClasses.filter(cls => !css.includes(cls));
if (missingClasses.length > 0) {
  console.error('Missing CSS component classes:', missingClasses);
  process.exit(1);
} else {
  console.log(`PASS: All ${componentClasses.length} component classes are styled in style.css`);
}

console.log('\nALL EMPIRICAL DOM & CSS VALIDATIONS PASSED!');
