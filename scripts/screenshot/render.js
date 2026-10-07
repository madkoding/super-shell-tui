// Renders HTML pages made by ans2html.py to PNG with headless Chromium.
// Usage: node render.js SCALE IN.html OUT.png [IN.html OUT.png ...]
const { chromium } = require('playwright');

(async () => {
  const [scale, ...pairs] = process.argv.slice(2);
  const browser = await chromium.launch(
    process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
  const page = await browser.newPage({ deviceScaleFactor: Number(scale) });
  for (let i = 0; i < pairs.length; i += 2) {
    await page.goto('file://' + require('path').resolve(pairs[i]));
    const box = await (await page.$('.w')).boundingBox();
    await page.setViewportSize({ width: Math.ceil(box.width + 56), height: Math.ceil(box.height + 56) });
    await page.screenshot({ path: pairs[i + 1] });
  }
  await browser.close();
})();
