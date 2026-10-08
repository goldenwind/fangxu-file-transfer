import test from 'node:test';
import assert from 'node:assert/strict';
import '../web/upload.js';

const { addFiles, remaining, summary, queuePage, transferEstimate, sendFile, maxFiles, maxBytes, pageSize } = globalThis.FangxuUpload;
const file = (name, size = 20, lastModified = 1) => ({ name, size, lastModified });
const id = () => 'a'.repeat(32);

test('repeated photo selections append new photos, deduplicate and preserve progress', () => {
  const queue = [];
  addFiles(queue, [file('旅行.jpg'), file('照片.HEIC')], id);
  queue[0].state = 'done';
  assert.deepEqual(addFiles(queue, [file('旅行.jpg'), file('新增.png')], id), { duplicate: 1, rejected: 0 });
  assert.equal(queue.length, 3);
  assert.equal(queue[0].state, 'done');
  assert.deepEqual(remaining(queue).map(entry => entry.file.name), ['照片.HEIC', '新增.png']);
});

test('queue limits preserve accepted files and allow adding after removal', () => {
  const queue = [];
  assert.deepEqual(addFiles(queue, Array.from({ length: maxFiles + 2 }, (_, n) => file(`${n}.jpg`)), id), { duplicate: 0, rejected: 2 });
  assert.equal(queue.length, 5000);
  queue.pop();
  assert.equal(addFiles(queue, [file('new.jpg')], id).rejected, 0);
  const large = [];
  assert.equal(addFiles(large, [file('oversize.mov', maxBytes + 1), file('large.mov', maxBytes), file('extra.jpg')], id).rejected, 1);
  assert.equal(large.length, 2);
});

test('a holiday queue accepts thousands of originals beyond 10 GB without losing deduplication', () => {
  const queue = [], originals = Array.from({ length: 3000 }, (_, n) => file(`IMG_${n}.HEIC`, 6 * 1024 ** 2));
  assert.deepEqual(addFiles(queue, originals, id), { duplicate: 0, rejected: 0 });
  assert.ok(summary(queue).bytes > 10 * 1024 ** 3);
  queue[0].state = 'done'; queue[105].state = 'failed';
  assert.deepEqual(addFiles(queue, [originals[0], file('new.jpg')], id), { duplicate: 1, rejected: 0 });
  assert.equal(remaining(queue).length, 3000);
  const view = queuePage(queue, 'all', 2);
  assert.equal(view.entries.length, pageSize);
  assert.equal(view.entries[0].file.name, 'IMG_100.HEIC');
  assert.equal(queuePage(queue, 'failed', 99).entries[0], queue[105]);
  assert.equal(queuePage(queue, 'unfinished').count, 3000);
  assert.equal(queuePage(queue, 'done').count, 1);
  queue[105].state = 'done';
  assert.deepEqual(queuePage(queue, 'failed', 99), { entries: [], page: 0, pages: 1, count: 0 });
});

test('time estimates exclude previously completed photos when a paused batch resumes', () => {
  const info = { bytes: 6000, loaded: 3000 };
  assert.equal(transferEstimate(info, 1000, 1000, 2000), null);
  assert.equal(transferEstimate(info, 1000, 3000, 6000), null);
  assert.deepEqual(transferEstimate(info, 1000, 1000, 6000), { bytesPerSecond: 400, secondsRemaining: 8 });
  assert.deepEqual(transferEstimate({ bytes: 6000, loaded: 6000 }, 1000, 1000, 6000), { bytesPerSecond: 1000, secondsRemaining: 0 });
});

test('retry selects failed and pending photos while completed bytes remain counted', () => {
  const queue = [];
  addFiles(queue, [file('done.jpg', 100), file('fail.jpg', 200), file('next.jpg', 300)], id);
  queue[0].state = 'done'; queue[1].state = 'failed';
  assert.deepEqual(summary(queue), { count: 3, done: 1, failed: 1, bytes: 600, loaded: 100 });
  assert.deepEqual(remaining(queue).map(entry => entry.file.name), ['fail.jpg', 'next.jpg']);
});

function fakeXHR({ status = 200, response = '{"status":"uploaded","files":["photo (1).jpg"]}', networkError = false } = {}) {
  const events = {}, uploads = {}, headers = {};
  return {
    upload: { addEventListener(name, handler) { uploads[name] = handler; } },
    addEventListener(name, handler) { events[name] = handler; },
    open(method, action) { this.method = method; this.action = action; },
    setRequestHeader(key, value) { headers[key] = value; },
    send(data) {
      this.data = data; this.headers = headers;
      uploads.progress({ lengthComputable: true, loaded: 100, total: 100 });
      this.status = status; this.responseText = response;
      queueMicrotask(() => events[networkError ? 'error' : 'load']());
    },
  };
}

test('upload only succeeds after a saved-file receipt and retains its retry identifier', async () => {
  const photo = new File(['image'], 'photo.jpg', { type: 'image/jpeg' });
  const entry = { file: photo, id: 'f'.repeat(32) }, progress = [];
  for (let attempt = 0; attempt < 2; attempt++) {
    const xhr = fakeXHR();
    assert.equal(await sendFile('/upload?upload_token=test', entry, bytes => progress.push(bytes), () => xhr), 'photo (1).jpg');
    assert.equal(xhr.headers['X-Upload-ID'], entry.id);
    assert.equal(xhr.headers.Accept, 'application/json');
    assert.equal(xhr.data.getAll('files').length, 1);
  }
  assert.deepEqual(progress, [photo.size, photo.size]);
});

test('network errors, expired links and malformed responses are retryable failures', async () => {
  const entry = { file: new File(['image'], 'photo.jpg'), id: id() };
  for (const [options, error] of [
    [{ networkError: true }, 'upload-network'],
    [{ status: 403, response: 'Forbidden' }, 'upload-unauthorized'],
    [{ response: '<html>service stopped</html>' }, 'upload-network'],
    [{ status: 500, response: '{"status":"upload-failed"}' }, 'upload-failed'],
    [{ response: '{"status":"uploaded","files":[]}' }, 'upload-network'],
  ]) {
    await assert.rejects(sendFile('/upload', entry, () => {}, () => fakeXHR(options)), value => value === error);
  }
});
