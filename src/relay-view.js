import { Channel } from '@tauri-apps/api/core';
import { escapeHTML as esc } from './helpers.js';

export const relayServices = {
  kiwi: { name: 'file.kiwi', detail: '加密中转', retention: '免费取件最长 1 天' },
  gofile: { name: 'Gofile', detail: '访客上传', retention: '连续 10 天无人下载后失效' },
  tmp0: { name: 'tmp0.cc', detail: '单文件 2 GB', retention: '默认有效期 7 天' },
  filebin: { name: 'Filebin', detail: '免登录', retention: '有效期约 7 天' },
  tempfile: { name: 'TempFile.org', detail: '单文件 100 MB', retention: '默认有效期 1 小时' },
};
const providersKey = 'fangxu-relay-providers';
const units = ['B', 'KB', 'MB', 'GB', 'TB'];

function formatSize(size, format) {
  const unit = Math.min(units.length - 1, Math.floor(Math.log10(Math.max(1, size)) / 3));
  return `${format.format(size / 1000 ** unit)} ${units[unit]}`;
}

function formatRetention({ file, provider, uploadedSize = file.size }, t) {
  if (provider !== 'kiwi') return t(relayServices[provider].retention);
  if (uploadedSize < 10e9) return t('免费取件有效期 1 天');
  return t('免费取件有效期 {hours} 小时', { hours: uploadedSize < 20e9 ? 12 : 6 });
}

export function setupRelayView(root, { invoke, desktop, t, notify, language }) {
  const form = root.querySelector('form');
  const picker = root.querySelector('#relay-choose');
  const files = root.querySelector('#relay-files');
  const submit = root.querySelector('#relay-submit');
  const cancel = root.querySelector('#relay-cancel');
  const providers = root.querySelector('#relay-providers');
  const resultsList = root.querySelector('#relay-results');
  const status = root.querySelector('#relay-status');
  let selection = [];
  let results = [];
  let selectionChange = null;
  let batch = null;

  try {
    const saved = JSON.parse(localStorage.getItem(providersKey));
    if (Array.isArray(saved)) {
      for (const input of providers.querySelectorAll('input')) input.checked = saved.includes(input.value);
    }
  } catch { /* Use the default if no saved preference can be read. */ }

  function selectedProviders() {
    return [...providers.querySelectorAll('input:checked')].map(input => input.value);
  }

  function canQueue(file, provider) {
    return !file.links[provider] && !results.some(result => result.file.id === file.id &&
      result.provider === provider && (result.state === 'queued' || result.state === 'active'));
  }

  function enqueue(targets, targetFiles = selection) {
    for (const provider of targets) {
      for (const file of targetFiles) {
        if (!canQueue(file, provider)) continue;
        const index = results.findIndex(result => result.file.id === file.id && result.provider === provider);
        const queued = { file, provider, state: 'queued' };
        if (index === -1) results.push(queued);
        else results[index] = queued;
      }
    }
  }

  function updateProgress(result, row) {
    const bar = row?.querySelector('progress');
    if (!bar || !result.progress) return;
    const { uploaded, speed } = result.progress;
    const total = result.file.size;
    const sent = Math.min(uploaded, total);
    const percent = total ? Math.floor(100 * (sent / total)) : 100;
    const format = new Intl.NumberFormat(language(), { maximumFractionDigits: 1 });
    bar.value = percent;
    if (batch?.active?.canceling) return;
    const details = t('已上传 {sent} / {total} · {percent}% · {speed}/s', {
      sent: formatSize(sent, format),
      total: formatSize(total, format),
      percent,
      speed: formatSize(speed, format),
    });
    row.querySelector('.helper').textContent = percent === 100 ? t('等待生成取件链接…') : details;
  }

  function render() {
    picker.disabled = Boolean(selectionChange || batch?.canceling);
    submit.hidden = !!batch;
    submit.disabled = !!selectionChange || !!batch || !selectedProviders().some(provider => selection.some(file => canQueue(file, provider)));
    cancel.hidden = !batch;
    cancel.disabled = !!batch?.canceling;
    providers.disabled = cancel.disabled;
    files.hidden = !selection.length;
    const sizes = new Intl.NumberFormat(language(), { maximumFractionDigits: 1 });
    files.innerHTML = selection.map(({ id, name, size }) => `
      <li><span>${esc(name)}</span><span class="relay-file-actions">${formatSize(size, sizes)}
        <button class="button secondary" type="button" data-remove="${esc(id)}"
          aria-label="${esc(t('移除 {file}', { file: name }))}" ${picker.disabled || batch?.active?.result.file.id === id ? 'disabled' : ''}>${esc(t('移除'))}</button>
      </span></li>`).join('');
    resultsList.hidden = !results.length;
    const finished = results.filter(result => result.state !== 'queued' && result.state !== 'active').length;
    status.textContent = batch ? t(batch.canceling ? '正在取消上传…' : '正在上传 · {done}/{total}', { done: finished, total: results.length })
      : results.length ? t('上传结束 · {done}/{total} 成功', { done: results.filter(result => result.url).length, total: results.length }) : '';
    resultsList.innerHTML = results.map((result, index) => {
      const { file, provider, state, url, error } = result;
      const active = state === 'active';
      const canceling = active && batch?.active?.canceling;
      const message = error || (canceling ? '正在取消上传…' : active ? '正在上传…' : '等待上传');
      return `
      <li class="relay-result">
        <div class="relay-result-heading"><strong>${esc(file.name)}</strong><span>${relayServices[provider].name}</span></div>
        ${url ? `
          <div class="relay-result-link">
            <a href="${esc(url)}" target="_blank" rel="noopener noreferrer" data-result="${index}">${esc(url)}</a>
            <button class="button secondary" type="button" data-copy="${index}">${esc(t('复制链接'))}</button>
          </div>
          <p class="helper">${esc(formatRetention(result, t))}</p>`
          : `${active ? `<progress class="relay-progress" max="100" aria-label="${esc(t('上传进度'))}"></progress>` : ''}
            <div class="relay-result-state">
              <p class="helper${error && state !== 'canceled' ? ' relay-error' : ''}">${esc(t(message))}</p>
              ${active ? `<button class="button stop-button" type="button" data-cancel-current ${canceling ? 'disabled' : ''}>${esc(t('取消当前上传'))}</button>` : ''}
            </div>`}
      </li>`;
    }).join('');
    const active = results.findIndex(result => result.state === 'active');
    if (active !== -1) updateProgress(results[active], resultsList.children[active]);
  }

  async function sendCancel(current, active) {
    try { await invoke('cancel_relay_upload', { uploadId: active.uploadId }); }
    catch (error) {
      if (batch !== current || current.active !== active) return;
      active.canceling = false;
      current.canceling = false;
      notify(error, true);
      render();
    }
  }

  function cancelActive(current) {
    const active = current?.active;
    if (!active || active.canceling) return;
    active.canceling = true;
    if (active.uploadId) void sendCancel(current, active);
  }

  async function upload(current, active) {
    const { result } = active;
    const uploadedSize = result.file.size;
    let events;
    const finished = new Promise(resolve => {
      events = new Channel(message => {
        if (!message.progress) {
          resolve(message);
          return;
        }
        if (result.state !== 'active') return;
        result.progress = message.progress;
        updateProgress(result, resultsList.children[results.indexOf(result)]);
      });
    });
    active.uploadId = await invoke('start_relay_upload', {
      provider: result.provider, fileId: result.file.id, events,
    });
    if (active.canceling) void sendCancel(current, active);
    const { url, error } = await finished;
    if (!url) throw error;
    result.file.links[result.provider] = url;
    result.url = url;
    result.uploadedSize = uploadedSize;
    result.state = 'complete';
  }

  function editSelection(change) {
    selectionChange = Promise.resolve().then(change).catch(error => notify(error, true)).finally(() => {
      selectionChange = null;
      render();
    });
    render();
  }

  form.hidden = !desktop;
  root.querySelector('#relay-desktop-only').hidden = desktop;
  picker.addEventListener('click', () => {
    if (picker.disabled) return;
    editSelection(async () => {
      const next = await invoke('choose_relay_files', { language: language() });
      if (!next) return;
      const previous = new Map(selection.map(file => [file.id, file]));
      selection = next.map(file => {
        const existing = previous.get(file.id);
        if (!existing) return file;
        // The picker snapshot may have been taken before an upload completed.
        return Object.assign(existing, { ...file, links: { ...file.links, ...existing.links } });
      });
      for (const file of selection) {
        for (const [provider, url] of Object.entries(file.links)) {
          if (!results.some(result => result.file.id === file.id && result.provider === provider)) {
            results.push({ file, provider, url, state: 'complete' });
          }
        }
      }
      if (batch && !batch.canceling) {
        enqueue(selectedProviders(), selection.filter(file => !previous.has(file.id)));
      }
    });
  });
  files.addEventListener('click', event => {
    const button = event.target.closest('[data-remove]');
    if (!button || button.disabled) return;
    const fileId = button.dataset.remove;
    editSelection(async () => {
      await invoke('remove_relay_file', { fileId });
      selection = selection.filter(file => file.id !== fileId);
      results = results.filter(result => result.file.id !== fileId);
    });
  });
  providers.addEventListener('change', event => {
    try { localStorage.setItem(providersKey, JSON.stringify(selectedProviders())); }
    catch { /* The choices still apply while this window is open. */ }
    if (batch && !batch.canceling) {
      const { value, checked } = event.target;
      if (checked) enqueue([value]);
      else results = results.filter(result => result.provider !== value || result.state !== 'queued');
    }
    render();
  });
  cancel.addEventListener('click', () => {
    if (!batch || batch.canceling) return;
    batch.canceling = true;
    for (const result of results) {
      if (result.state !== 'queued') continue;
      result.state = 'canceled';
      result.error = '上传已取消';
    }
    cancelActive(batch);
    render();
  });
  form.addEventListener('submit', async event => {
    event.preventDefault();
    if (submit.disabled) return;
    enqueue(selectedProviders());
    const current = { active: null, canceling: false };
    batch = current;
    let currentProvider;
    while (!current.canceling) {
      // A pending removal must finish before its first queued upload can start.
      if (selectionChange) await selectionChange;
      if (current.canceling) break;
      const queued = results.filter(item => item.state === 'queued');
      const result = queued.find(item => item.provider === currentProvider) ?? queued[0];
      if (!result) break;
      currentProvider = result.provider;
      const active = { result, uploadId: null, canceling: false };
      current.active = active;
      result.state = 'active';
      render();
      try { await upload(current, active); }
      catch (error) {
        result.error = typeof error === 'string' ? error : '无法连接中转服务，请重试';
        result.state = result.error === '上传已取消' ? 'canceled' : 'failed';
      }
      current.active = null;
      if (selectionChange) render();
    }
    batch = null;
    render();
  });
  resultsList.addEventListener('click', async event => {
    const link = event.target.closest('[data-result]');
    const copy = event.target.closest('[data-copy]');
    if (event.target.closest('[data-cancel-current]')) {
      cancelActive(batch);
      render();
    } else if (link) {
      event.preventDefault();
      try { await invoke('open_relay_link', { link: results[Number(link.dataset.result)].url }); }
      catch (error) { notify(error, true); }
    } else if (copy) {
      try {
        await navigator.clipboard.writeText(results[Number(copy.dataset.copy)].url);
        notify('取件链接已复制');
      } catch { notify('无法复制链接', true); }
    }
  });
  return render;
}
