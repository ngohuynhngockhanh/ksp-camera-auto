const {chromium} = require('playwright');
(async () => {
  const b = await chromium.launch();
  const p = await b.newPage({viewport:{width:1280,height:900}});
  await p.goto('https://apphub.a.inut.vn/citizen', {waitUntil:'networkidle'});
  await p.fill('#login input[name=phone]','0961197999');
  await p.fill('#login input[name=password]','VNMap12345@@@');
  await p.click('#login button[type=submit]');
  await p.waitForSelector('.sample', {timeout:20000});
  const chips = await p.$$eval('.sample', els => els.map(e => e.textContent));
  console.log('CHIPS:', JSON.stringify(chips, null, 1));
  const box = await p.$eval('.sample', e => { const r=e.getBoundingClientRect(); return {h:Math.round(r.height), fs:getComputedStyle(e).fontSize}; });
  console.log('chip height/font:', JSON.stringify(box));
  await p.screenshot({path:'/tmp/samples_desktop.png', fullPage:false});
  // click first chip -> must send question
  await p.click('.sample');
  await p.waitForSelector('.bubble', {timeout:20000});
  const first = await p.$eval('.bubble', e => e.textContent.slice(0,60));
  console.log('SENT BUBBLE:', first);
  const gone = await p.$('.welcome');
  console.log('welcome removed after ask:', gone === null);
  await p.waitForTimeout(25000);
  await p.screenshot({path:'/tmp/samples_answer.png', fullPage:false});
  // mobile
  const m = await b.newPage({viewport:{width:390,height:844}});
  await m.goto('https://apphub.a.inut.vn/citizen', {waitUntil:'networkidle'});
  await m.fill('#login input[name=phone]','0961197999');
  await m.fill('#login input[name=password]','VNMap12345@@@');
  await m.click('#login button[type=submit]');
  await m.waitForSelector('.sample', {timeout:20000});
  const ov = await m.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
  console.log('mobile horizontal overflow:', ov);
  await m.screenshot({path:'/tmp/samples_mobile.png', fullPage:false});
  await b.close();
})();
