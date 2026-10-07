import test from 'node:test';
import assert from 'node:assert/strict';
import { initialLanguage, languageStorageKey, translate, translateError } from '../src/i18n.js';

test('saved language wins over system language; missing or unusable storage falls back safely', () => {
  const storage = { getItem(key) { assert.equal(key, languageStorageKey); return 'en'; } };
  assert.equal(initialLanguage(storage, 'zh-CN'), 'en');
  assert.equal(initialLanguage({ getItem: () => 'zh' }, 'en-US'), 'zh');
  assert.equal(initialLanguage({ getItem: () => 'invalid' }, 'zh-TW'), 'zh');
  assert.equal(initialLanguage(undefined, 'en-US'), 'en');
  assert.equal(initialLanguage({ getItem() { throw new Error('disabled'); } }, 'zh-CN'), 'zh');
});

test('live status interpolation translates labels without altering ports and paths', () => {
  assert.equal(translate('en', '服务运行中 · 端口 {port} · {protection}', { port: 54321, protection: translate('en', '令牌保护已开启') }), 'Running · Port 54321 · Token protection on');
  assert.equal(translate('zh', '正在端口 {port} 上共享文件，扫码即可开始。', { port: 54321 }), '正在端口 54321 上共享文件，扫码即可开始。');
  assert.equal(translate('en', '/Users/demo/中文文件'), '/Users/demo/中文文件');
});

test('English errors retain actionable OS details and do not partially translate unrelated text', () => {
  const error = '共享目录不可用，请重新选择: stat /Users/demo/中文文件: no such file or directory';
  assert.equal(translateError('en', error), 'Shared folder unavailable. Choose another folder: stat /Users/demo/中文文件: no such file or directory');
  assert.equal(translateError('zh', error), error);
  assert.equal(translateError('en', 'port 8000: address in use'), 'port 8000: address in use');
  assert.equal(translateError('en', '请选择或输入共享目录'), 'Choose or enter a shared folder.');
});
