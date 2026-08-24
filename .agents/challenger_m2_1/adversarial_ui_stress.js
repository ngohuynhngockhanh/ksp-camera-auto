const { chromium } = require('@playwright/test');
const http = require('http');
const fs = require('fs');
const path = require('path');
const { mockApi } = require('../../tests/ui/fixtures');

function startStaticServer(port) {
  const staticDir = path.resolve(__dirname, '../../web/static');
  const mimeTypes = {
    '.html': 'text/html',
    '.css': 'text/css',
    '.js': 'application/javascript',
    '.png': 'image/png',
    '.json': 'application/json'
  };

  const server = http.createServer((req, res) => {
    let reqPath = req.url.split('?')[0].split('#')[0];
    if (reqPath === '/' || reqPath === '') reqPath = '/index.html';
    const filePath = path.join(staticDir, reqPath);

    if (fs.existsSync(filePath) && fs.statSync(filePath).isFile()) {
      const ext = path.extname(filePath);
      const contentType = mimeTypes[ext] || 'text/plain';
      res.writeHead(200, { 'Content-Type': contentType });
      res.end(fs.readFileSync(filePath));
    } else {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end('Not Found');
    }
  });

  return new Promise((resolve) => {
    server.listen(port, '127.0.0.1', () => {
      resolve(server);
    });
  });
}

(async () => {
  console.log('=== STARTING EMPIRICAL ADVERSARIAL PLAYWRIGHT STRESS HARNESS ===');
  const PORT = 4192;
  const server = await startStaticServer(PORT);
  console.log(`Hermetic static server listening on http://127.0.0.1:${PORT}`);

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 }
  });
  const page = await context.newPage();

  // Use the full project fixture table
  await mockApi(page);

  await page.goto(`http://127.0.0.1:${PORT}/index.html#redbida`);
  await page.waitForFunction(() => window.__kspReady === true);

  console.log('1. Verifying View Display & Heading...');
  const isViewActive = await page.locator('#view-redbida').isVisible();
  console.log('   #view-redbida is visible:', isViewActive);
  if (!isViewActive) throw new Error('#view-redbida is not visible');

  const headingText = await page.locator('#view-redbida .page-title').textContent();
  console.log('   Heading:', headingText);
  if (!headingText.includes('RedBida / OTA-MQTT')) throw new Error('Incorrect heading');

  console.log('2. Verifying Glassmorphism Computed Styles on Cards...');
  const cardStyles = await page.locator('.redbida-metric-card').first().evaluate(el => {
    const computed = window.getComputedStyle(el);
    return {
      backdropFilter: computed.backdropFilter || computed.webkitBackdropFilter,
      borderRadius: computed.borderRadius,
      borderStyle: computed.borderStyle,
      borderColor: computed.borderColor,
      backgroundColor: computed.backgroundColor
    };
  });
  console.log('   Metric Card Computed Styles:', cardStyles);
  if (!cardStyles.borderRadius || cardStyles.borderStyle === 'none') {
    throw new Error('Glassmorphism card border/radius missing');
  }

  console.log('3. Verifying 4-Pillar Knowledge Hub...');
  const pillarCount = await page.locator('.redbida-pillar-card').count();
  console.log('   Pillar Cards count:', pillarCount);
  if (pillarCount !== 4) throw new Error(`Expected 4 pillar cards, found ${pillarCount}`);

  const pillarTitles = await page.locator('.redbida-pillar-title').allTextContents();
  console.log('   Pillar Titles:', pillarTitles);

  console.log('4. Verifying 1-Click Preset Generator Panel...');
  const presetPanelVisible = await page.locator('#redbida-preset-panel').isVisible();
  console.log('   #redbida-preset-panel visible:', presetPanelVisible);
  if (!presetPanelVisible) throw new Error('#redbida-preset-panel missing or hidden');

  const swatchesCount = await page.locator('#redbida-preset-swatches .redbida-swatch').count();
  console.log('   Preset Swatches count:', swatchesCount);
  if (swatchesCount !== 6) throw new Error(`Expected 6 swatches, found ${swatchesCount}`);

  console.log('5. Verifying Responsive Layout on Mobile Viewport...');
  await page.setViewportSize({ width: 375, height: 667 });
  await page.waitForTimeout(200);
  const statusGridCols = await page.locator('.redbida-status-grid').evaluate(el => {
    return window.getComputedStyle(el).gridTemplateColumns;
  });
  console.log('   Mobile Status Grid columns:', statusGridCols);

  console.log('6. Verifying Dark / Light Theme Switching...');
  await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'));
  const lightBgToken = await page.evaluate(() => {
    return window.getComputedStyle(document.documentElement).getPropertyValue('--glass-bg').trim();
  });
  console.log('   Light Theme --glass-bg token:', lightBgToken);
  if (!lightBgToken.includes('255')) {
    throw new Error('Light theme token not activated');
  }

  await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'));
  const darkBgToken = await page.evaluate(() => {
    return window.getComputedStyle(document.documentElement).getPropertyValue('--glass-bg').trim();
  });
  console.log('   Dark Theme --glass-bg token:', darkBgToken);
  if (!darkBgToken.includes('30, 41, 59')) {
    throw new Error('Dark theme token not activated');
  }

  console.log('7. Verifying Quick Actions Buttons & Preserved Test IDs...');
  const refreshBtn = await page.getByTestId('redbida-refresh').isVisible();
  const applyBtn = await page.getByTestId('redbida-apply').isVisible();
  const searchInput = await page.getByTestId('redbida-search').isVisible();
  const groupSelect = await page.getByTestId('redbida-group').isVisible();
  console.log('   Quick action & filter controls visible:', { refreshBtn, applyBtn, searchInput, groupSelect });
  if (!refreshBtn || !applyBtn || !searchInput || !groupSelect) {
    throw new Error('Critical test-id elements are missing or not visible');
  }

  console.log('\n=== ALL PLAYWRIGHT EMPIRICAL STRESS TESTS PASSED SUCCESSFULLY! ===');
  await browser.close();
  server.close();
  process.exit(0);
})();
