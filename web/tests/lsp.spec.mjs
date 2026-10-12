import { test, expect } from '@playwright/test';

const pageErrors = new WeakMap();

test.beforeEach(async ({ page }) => {
  const errors = [];
  pageErrors.set(page, errors);
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await expect(page.locator('#status')).toHaveText('Ready · SQLite');
  await page.evaluate(() => window.playground.ready);
  expect(errors).toEqual([]);
});

test.afterEach(async ({ page }) => {
  expect(pageErrors.get(page)).toEqual([]);
});

test('live diagnostics and clearing after editing', async ({ page }) => {
  await page.selectOption('#example', 'lint');
  await expect(page.locator('#diagnostics')).toContainText('SELECT *');
  await page.evaluate(() => window.playground.model.setValue('SELECT name FROM users;'));
  await expect(page.locator('#diagnostic-count')).toHaveText('0');
});

test('WHERE diagnostics accept SQLite truth values and flag missing operands', async ({ page }) => {
  await page.evaluate(() => window.playground.model.setValue('SELECT u.name, u.email\nFROM users AS u\nWHERE u.id ;'));
  await expect(page.locator('#diagnostic-count')).toHaveText('0');
  await page.click('#execute');
  await expect(page.locator('#result')).toContainText('Ada');
  await page.evaluate(() => window.playground.model.setValue('SELECT u.name, u.email\nFROM users AS u\nWHERE u.id = ;'));
  await expect(page.locator('#diagnostics')).toContainText('Incomplete predicate');
  const markers = await page.evaluate(() => monaco.editor.getModelMarkers({ owner: 'sqls' }));
  expect(markers.some(marker => marker.code === 'syntax-error' && marker.startLineNumber === 3)).toBe(true);
  await page.evaluate(() => window.playground.model.setValue('SELECT u.name, u.email\nFROM users AS u\nWHERE u.id = 1;'));
  await expect(page.locator('#diagnostic-count')).toHaveText('0');
});

test('LSP completion appears in the editor and inserts a column', async ({ page }) => {
  await page.evaluate(() => {
    const { editor, model } = window.playground;
    model.setValue('SELECT u. FROM users AS u;');
    editor.setPosition({ lineNumber: 1, column: 10 });
    editor.focus();
    editor.trigger('test', 'editor.action.triggerSuggest', {});
  });
  await expect(page.locator('.suggest-widget')).toBeVisible();
  await expect(page.locator('.suggest-widget')).toContainText('name');
  await page.keyboard.press('Enter');
  expect(await page.evaluate(() => window.playground.model.getValue())).toMatch(/^SELECT u\.\w+ FROM/);
  await expect(page.locator('#log')).toContainText('textDocument/completion');
});

test('format document and selection apply LSP edits', async ({ page }) => {
  await page.evaluate(() => window.playground.model.setValue('select   name,email from users;'));
  await page.click('#format');
  await expect.poll(() => page.evaluate(() => window.playground.model.getValue())).toMatch(/^SELECT\n/);
  await page.evaluate(() => {
    const { editor, model } = window.playground;
    model.setValue('select 1;\nselect   name from users;\nselect 2;');
    editor.setSelection({ startLineNumber: 2, startColumn: 1, endLineNumber: 2, endColumn: 25 });
  });
  await page.click('#range-format');
  await expect.poll(() => page.evaluate(() => window.playground.model.getValue())).toMatch(/^select 1;\nSELECT\n/);
  expect(await page.evaluate(() => window.playground.model.getValue())).toMatch(/\nselect 2;$/);
});

test('hover displays SQLite column metadata', async ({ page }) => {
  await page.evaluate(() => {
    const { editor, model } = window.playground;
    model.setValue('SELECT name FROM users;');
    editor.setPosition({ lineNumber: 1, column: 9 });
    editor.focus();
    editor.trigger('test', 'editor.action.showHover', {});
  });
  await expect(page.locator('.monaco-hover')).toContainText('TEXT');
  await expect(page.locator('#log')).toContainText('textDocument/hover');
});

test('F2 rename preserves Unicode and uses the current document version', async ({ page }) => {
  await page.selectOption('#example', 'unicode');
  await page.evaluate(() => {
    const { editor, model } = window.playground;
    editor.setPosition({ lineNumber: 1, column: model.getValue().indexOf('u.name') + 1 });
    editor.focus();
  });
  await page.keyboard.press('F2');
  const input = page.locator('.rename-box input');
  await expect(input).toBeVisible();
  await input.fill('customer');
  await input.press('Enter');
  await expect.poll(() => page.evaluate(() => window.playground.model.getValue()))
    .toBe("SELECT '😀', customer.name FROM users AS customer;");
});

test('definition navigates to the alias declaration', async ({ page }) => {
  await page.evaluate(() => {
    const { editor, model } = window.playground;
    model.setValue('SELECT u.name\nFROM users AS u;');
    editor.setPosition({ lineNumber: 1, column: 8 });
    editor.focus();
    editor.trigger('test', 'editor.action.revealDefinition', {});
  });
  await expect.poll(() => page.evaluate(() => window.playground.editor.getPosition().lineNumber)).toBe(2);
});

test('signature help identifies the second INSERT parameter', async ({ page }) => {
  await page.selectOption('#example', 'insert');
  await page.evaluate(() => {
    const { editor, model } = window.playground;
    editor.setPosition({ lineNumber: 2, column: model.getLineContent(2).indexOf(')') + 1 });
    editor.focus();
    editor.trigger('test', 'editor.action.triggerParameterHints', {});
  });
  await expect(page.locator('.parameter-hints-widget')).toBeVisible();
  await expect(page.locator('.parameter-hints-widget')).toContainText('email');
  await expect(page.locator('#log')).toContainText('textDocument/signatureHelp');
});

test('query execution and restart use real server processes', async ({ page }, testInfo) => {
  await page.click('#execute');
  await expect(page.locator('#result')).toContainText('Ada');
  await expect(page.locator('#result')).toContainText('ada@example.com');
  await page.click('#restart');
  await expect(page.locator('#restart')).toBeEnabled();
  await expect(page.locator('#status')).toHaveText('Ready · SQLite');
  await expect(page.locator('#log')).toContainText('shutdown');
  await expect(page.locator('#log')).toContainText('exit');
  await page.click('#execute');
  await expect(page.locator('#result')).toContainText('Ada');
  await page.screenshot({ path: testInfo.outputPath('playground.png'), fullPage: true });
});
