import test from 'node:test';
import assert from 'node:assert/strict';
import { escapeHTML, readSettings } from '../src/helpers.js';

test('paths and connection URLs are safe to render as HTML attributes', () => {
  assert.equal(escapeHTML('"<&\'雪>'), '&quot;&lt;&amp;&#39;雪&gt;');
});
test('settings accept automatic and maximum ports and preserve Unicode paths', () => {
  const base = { directory: ' /Users/用户/共享文件 ', protected: true, autoStart: false };
  assert.deepEqual(readSettings({ ...base, port: '0' }), { directory: '/Users/用户/共享文件', port: 0, protected: true, autoStart: false });
  assert.equal(readSettings({ ...base, port: '65535' }).port, 65535);
  for (const port of ['', '-1', '65536', '1.5', 'abc']) assert.throws(() => readSettings({ ...base, port }));
  assert.throws(() => readSettings({ ...base, directory: ' ', port: '0' }));
});
