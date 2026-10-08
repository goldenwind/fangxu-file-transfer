/* Shared by the embedded browser page and Node's queue tests. No third-party assets. */
(() => {
  // Each file has its own request; the request size limit is not a batch limit.
  const maxFiles = 5000, maxBytes = 10 * 1024 ** 3 - 1024 ** 2, pageSize = 50;
  const fileKey = file => JSON.stringify([file.name, file.size, file.lastModified]);
  const newID = () => {
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    return Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('');
  };
  function addFiles(queue, files, makeID = newID) {
    const keys = new Set(queue.map(entry => fileKey(entry.file)));
    let duplicate = 0, rejected = 0;
    for (const file of files) {
      const key = fileKey(file);
      if (keys.has(key)) { duplicate++; continue; }
      if (queue.length >= maxFiles || file.size > maxBytes) { rejected++; continue; }
      queue.push({ file, id: makeID(), state: 'pending', loaded: 0 });
      keys.add(key);
    }
    return { duplicate, rejected };
  }
  const remaining = queue => queue.filter(entry => entry.state === 'pending' || entry.state === 'failed');
  const summary = queue => ({
    count: queue.length,
    done: queue.filter(entry => entry.state === 'done').length,
    failed: queue.filter(entry => entry.state === 'failed').length,
    bytes: queue.reduce((sum, entry) => sum + entry.file.size, 0),
    loaded: queue.reduce((sum, entry) => sum + (entry.state === 'done' ? entry.file.size : entry.loaded), 0),
  });
  const size = bytes => bytes < 1024 ? bytes + ' B' : bytes < 1024 ** 2 ? (bytes / 1024).toFixed(1) + ' KB' : bytes < 1024 ** 3 ? (bytes / 1024 ** 2).toFixed(1) + ' MB' : (bytes / 1024 ** 3).toFixed(2) + ' GB';
  function queuePage(queue, filter = 'all', page = 0) {
    const entries = queue.filter(entry => filter === 'unfinished' ? entry.state !== 'done' : filter === 'failed' || filter === 'done' ? entry.state === filter : true);
    const pages = Math.max(1, Math.ceil(entries.length / pageSize));
    page = Math.max(0, Math.min(page, pages - 1));
    return { entries: entries.slice(page * pageSize, (page + 1) * pageSize), page, pages, count: entries.length };
  }
  function transferEstimate(info, startedAt, startLoaded, now) {
    const elapsed = (now - startedAt) / 1000, transferred = info.loaded - startLoaded;
    if (elapsed < 3 || transferred <= 0) return null;
    const bytesPerSecond = transferred / elapsed;
    return { bytesPerSecond, secondsRemaining: Math.ceil(Math.max(0, info.bytes - info.loaded) / bytesPerSecond) };
  }

  function sendFile(action, entry, onProgress, createXHR = () => new XMLHttpRequest()) {
    return new Promise((resolve, reject) => {
      const xhr = createXHR(), data = new FormData();
      let timer;
      const finish = (error, result) => { clearTimeout(timer); error ? reject(error) : resolve(result); };
      const watch = () => { clearTimeout(timer); timer = setTimeout(() => xhr.abort(), 120000); };
      xhr.upload.addEventListener('progress', event => {
        watch();
        if (event.lengthComputable) onProgress(Math.min(entry.file.size, entry.file.size * event.loaded / event.total));
      });
      xhr.upload.addEventListener('load', watch);
      xhr.addEventListener('load', () => {
        let result;
        try { result = JSON.parse(xhr.responseText); } catch (_) { /* A stopped service may return HTML. */ }
        if (xhr.status === 200 && result?.status === 'uploaded' && result.files?.length === 1) finish(null, result.files[0]);
        else finish((result?.status !== 'uploaded' && result?.status) || (xhr.status === 403 ? 'upload-unauthorized' : 'upload-network'));
      });
      xhr.addEventListener('error', () => finish('upload-network'));
      xhr.addEventListener('abort', () => finish('upload-network'));
      xhr.open('POST', action);
      xhr.setRequestHeader('Accept', 'application/json');
      xhr.setRequestHeader('X-Upload-ID', entry.id);
      data.append('files', entry.file, entry.file.name);
      watch();
      try { xhr.send(data); } catch (_) { finish('upload-network'); }
    });
  }

  function mount({ t, translatedText, onSaved = async () => {} }) {
    const $ = selector => document.querySelector(selector);
    const form = $('.upload-form'), picker = $('#upload-files'), photos = $('#upload-photos');
    const selection = $('#upload-selection'), submit = $('.upload-submit'), clear = $('#upload-clear');
    const pause = $('#upload-pause'), list = $('#upload-queue'), status = $('#upload-status');
    const progress = $('#upload-progress'), progressText = $('#upload-progress-text'), refresh = $('#upload-refresh');
    const filter = $('#upload-filter'), pager = $('#upload-pages'), pageText = $('#upload-page-text');
    const previous = $('#upload-previous'), next = $('#upload-next'), current = $('#upload-current');
    const empty = $('#upload-queue-empty'), activity = $('#upload-activity'), estimate = $('#upload-estimate');
    const queue = [], rows = new Map();
    let running = false, pauseRequested = false, notice = '', noticeValues = {}, page = 0;
    let activeEntry = null, startedAt = 0, startLoaded = 0, renderTimer, clockTimer;
    const states = { pending: '待上传', uploading: '正在上传', done: '已保存到电脑', failed: '上传失败' };
    const errors = {
      'upload-network': '连接中断或等待超时，请检查 Wi-Fi 和电脑服务后重试。',
      'upload-unauthorized': '访问链接已失效，请重新扫描电脑上的二维码。',
      'upload-invalid': '文件名无效，请移除此文件后继续。',
      'upload-too-large': '单个文件需小于 10 GB，请移除此文件后继续。',
      'upload-failed': '电脑未能保存文件，请检查共享目录和磁盘空间后重试。',
    };
    const showNotice = (key, values = {}) => { notice = key; noticeValues = values; status.hidden = !key; translatedText(status, key, values); };
    const scheduleRender = () => { renderTimer ||= setTimeout(render, 100); };
    function releaseRows() {
      for (const row of rows.values()) if (row.url) URL.revokeObjectURL(row.url);
      rows.clear();
      list.replaceChildren();
    }
    function render() {
      clearTimeout(renderTimer); renderTimer = undefined;
      const info = summary(queue), pending = remaining(queue);
      selection.textContent = info.count ? t('已选择 {count} 个文件 · {size}', { count: info.count, size: size(info.bytes) }) : t('尚未选择');
      submit.disabled = running || pending.length === 0;
      submit.textContent = t(running ? '正在上传…' : info.done === info.count && info.count ? '上传完成' : info.failed ? '重试失败并继续' : info.done ? '继续上传' : '开始上传');
      clear.disabled = running || !info.count;
      clear.textContent = t(info.done === info.count && info.count ? '清空记录' : '清空列表');
      pause.hidden = !running;
      pause.disabled = pauseRequested;
      pause.textContent = t(pauseRequested ? '当前文件完成后暂停' : '暂停上传');
      refresh.hidden = running || !info.done;
      refresh.textContent = t('查看电脑已收到的文件');
      progress.hidden = !info.count;
      progress.value = info.bytes ? Math.round(info.loaded / info.bytes * 100) : info.done === info.count ? 100 : 0;
      if (info.done < info.count) progress.value = Math.min(99, progress.value);
      progressText.hidden = !info.count;
      progressText.textContent = t('已保存 {done} / {count} · {percent}%', { done: info.done, count: info.count, percent: progress.value });
      activity.hidden = !running || !activeEntry;
      activity.textContent = activeEntry ? t('正在上传：{name}', { name: activeEntry.file.name }) : '';
      const rate = running ? transferEstimate(info, startedAt, startLoaded, performance.now()) : null;
      estimate.hidden = !running;
      estimate.textContent = !rate ? t('正在估算速度和剩余时间…') : t('{speed}/秒 · 预计剩余 {minutes} 分钟', { speed: size(rate.bytesPerSecond), minutes: Math.max(1, Math.ceil(rate.secondsRemaining / 60)) });
      filter.hidden = pager.hidden = !info.count;
      const counts = { all: info.count, unfinished: info.count - info.done, failed: info.failed, done: info.done };
      const labels = { all: '全部', unfinished: '未完成', failed: '上传失败', done: '已保存到电脑' };
      for (const option of filter.options) option.textContent = t(labels[option.value]) + ' (' + counts[option.value] + ')';
      const view = queuePage(queue, filter.value, page); page = view.page;
      pageText.textContent = t('第 {page} / {pages} 页 · {count} 个', { page: page + 1, pages: view.pages, count: view.count });
      previous.disabled = page === 0; next.disabled = page === view.pages - 1;
      current.hidden = !running || !activeEntry;
      list.hidden = !view.entries.length;
      empty.hidden = !info.count || !!view.entries.length;
      if (notice) translatedText(status, notice, noticeValues);
      if (rows.size !== view.entries.length || view.entries.some(entry => !rows.has(entry))) {
        releaseRows();
        for (const entry of view.entries) {
          const row = document.createElement('li'); row.className = 'upload-row';
          const preview = document.createElement('span'); preview.className = 'upload-preview'; preview.textContent = '↑';
          let url;
          // HEIC/HEIF remains uploadable even when the browser cannot decode its preview.
          if (/^image\/(jpeg|png|webp|gif|avif)$/.test(entry.file.type)) {
            const image = document.createElement('img'); image.alt = ''; image.loading = 'lazy'; image.decoding = 'async';
            url = URL.createObjectURL(entry.file); image.src = url;
            image.addEventListener('error', () => image.remove()); preview.append(image);
          }
          const details = document.createElement('span'); details.className = 'upload-row-details';
          const name = document.createElement('strong'); name.textContent = entry.file.name;
          const meta = document.createElement('span'), result = document.createElement('span'); result.className = 'upload-row-result';
          details.append(name, meta, result);
          const remove = document.createElement('button'); remove.type = 'button'; remove.className = 'upload-remove'; remove.textContent = '×';
          remove.addEventListener('click', () => {
            if (running) return;
            queue.splice(queue.indexOf(entry), 1);
            showNotice(''); render();
          });
          row.append(preview, details, remove); list.append(row);
          rows.set(entry, { row, name, meta, result, remove, url });
        }
      }
      for (const entry of view.entries) {
        const { row, name, meta, result, remove } = rows.get(entry);
        row.dataset.state = entry.state;
        name.textContent = entry.savedName || entry.file.name;
        const saving = entry.state === 'uploading' && entry.loaded >= entry.file.size;
        meta.textContent = size(entry.file.size) + ' · ' + t(saving ? '电脑正在确认保存' : states[entry.state]) + (entry.state === 'uploading' && !saving ? ' ' + Math.floor(entry.loaded / Math.max(1, entry.file.size) * 100) + '%' : '');
        result.textContent = entry.error ? t(errors[entry.error] || errors['upload-failed']) : entry.savedName && entry.savedName !== entry.file.name ? t('同名文件已自动重命名') : '';
        remove.disabled = running;
        remove.setAttribute('aria-label', t('移除 {name}', { name: entry.file.name }));
      }
    }
    function selectFiles(input) {
      const result = addFiles(queue, Array.from(input.files)); input.value = '';
      if (result.rejected) showNotice('有 {count} 个文件未加入：列表最多 5,000 个文件，单个需小于 10 GB。可传完后清空记录，再添加下一批。', { count: result.rejected });
      else if (result.duplicate) showNotice('已跳过 {count} 个重复选择的文件。', { count: result.duplicate });
      else showNotice(running ? '上传期间请保持页面在前台，避免锁屏或切换 Wi-Fi。' : '');
      render();
    }
    picker.addEventListener('change', () => selectFiles(picker));
    photos.addEventListener('change', () => selectFiles(photos));
    clear.addEventListener('click', () => {
      if (running) return;
      queue.length = 0; page = 0; filter.value = 'all'; showNotice(''); render();
    });
    filter.addEventListener('change', () => { page = 0; list.scrollTop = 0; render(); });
    previous.addEventListener('click', () => { page--; list.scrollTop = 0; render(); });
    next.addEventListener('click', () => { page++; list.scrollTop = 0; render(); });
    current.addEventListener('click', () => {
      if (!activeEntry) return;
      filter.value = 'all'; page = Math.floor(queue.indexOf(activeEntry) / pageSize); render();
      rows.get(activeEntry)?.row.scrollIntoView({ block: 'nearest' });
    });
    pause.addEventListener('click', () => { pauseRequested = true; render(); });
    refresh.addEventListener('click', async event => {
      event.preventDefault();
      try { await onSaved(); } catch (_) { /* Keep the queue when the service is unavailable. */ }
      $('#list').scrollIntoView({ behavior: 'smooth', block: 'start' });
    });
    form.addEventListener('submit', async event => {
      event.preventDefault();
      if (running || !remaining(queue).length) return;
      running = true; pauseRequested = false;
      startedAt = performance.now(); startLoaded = summary(queue).loaded;
      clockTimer = setInterval(scheduleRender, 1000);
      showNotice('上传期间请保持页面在前台，避免锁屏或切换 Wi-Fi。'); render();
      let connectionError = false;
      try {
        // The iterator includes photos added while the batch is uploading.
        for (const entry of queue) {
          if (pauseRequested) break;
          if (entry.state === 'done') continue;
          activeEntry = entry;
          entry.state = 'uploading'; entry.loaded = 0; entry.error = ''; render();
          try {
            entry.savedName = await sendFile(form.action, entry, loaded => { entry.loaded = loaded; scheduleRender(); });
            entry.state = 'done'; entry.loaded = entry.file.size;
          } catch (error) {
            entry.state = 'failed'; entry.loaded = 0; entry.error = error;
            if (error === 'upload-network' || error === 'upload-unauthorized') { connectionError = true; break; }
          }
          render();
        }
      } finally {
        running = false; activeEntry = null; clearInterval(clockTimer);
        const info = summary(queue);
        if (connectionError) showNotice('连接异常，已保留列表和成功记录。检查连接后可重试。');
        else if (pauseRequested && remaining(queue).length) showNotice('已暂停，已保存的文件会保留在电脑上。点击继续上传。');
        else if (info.failed) showNotice('已保存 {done} 个，失败 {failed} 个。可重试失败文件。', info);
        else showNotice('全部 {count} 个文件已保存到电脑的共享目录。', info);
        render();
        if (info.done) {
          try { await onSaved(); } catch (_) { /* The received-files link remains available. */ }
        }
      }
    });
    window.addEventListener('beforeunload', event => {
      if (running || remaining(queue).length) { event.preventDefault(); event.returnValue = ''; }
    });
    render();
    return { render };
  }
  globalThis.FangxuUpload = { addFiles, remaining, summary, queuePage, transferEstimate, sendFile, mount, maxFiles, maxBytes, pageSize };
})();
