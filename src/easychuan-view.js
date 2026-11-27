import qrcode from 'qrcode-generator';
import { escapeHTML as esc } from './helpers.js';

const maxFiles = 10;

const messages = {
  sending: '正在发送 {file} · {percent}%',
  complete: '接收方已收取文件',
  receiveFailed: '接收方未能保存文件，可重新下载。',
  interrupted: '传输中断，可在接收端重新下载。',
  viewing: '接收方正在观看',
};

const routes = {
  direct: '连接方式：P2P 直传',
  relay: '连接方式：TURN 中继',
  unknown: '连接方式：尚未识别',
};

async function captureMedia(kind, audio) {
  const screen = kind === 'screen';
  const devices = navigator.mediaDevices;
  const capture = screen ? devices?.getDisplayMedia : devices?.getUserMedia;
  if (typeof RTCPeerConnection === 'undefined' || !capture) {
    throw new Error(screen ? '当前系统 WebView 不支持屏幕共享' : '当前系统 WebView 不支持摄像头共享');
  }
  const video = screen ? { frameRate: { ideal: 30 } }
    : { width: { ideal: 1280 }, height: { ideal: 720 }, frameRate: { ideal: 30 } };
  try {
    return await capture.call(devices, { video, audio });
  } catch (error) {
    if (['NotAllowedError', 'AbortError'].includes(error.name)) {
      throw new Error(screen ? '屏幕共享已取消或未获授权' : '请允许访问摄像头或麦克风后重试');
    }
    if (error.name === 'NotFoundError' || error.name === 'NotReadableError') {
      throw new Error(screen ? '无法读取所选屏幕或窗口' : '找不到可用的摄像头或麦克风');
    }
    throw error;
  }
}

export function setupEasyChuanView(view, { register, registerText, registerMedia, translate: t, notify }) {
  const modes = view.querySelector('.easychuan-modes');
  const input = view.querySelector('#easychuan-files');
  const choose = view.querySelector('#easychuan-choose');
  const fileCount = view.querySelector('#easychuan-file-count');
  const fileList = view.querySelector('#easychuan-file-list');
  const editDialog = view.querySelector('#easychuan-edit-dialog');
  const text = view.querySelector('#easychuan-text');
  const audio = view.querySelector('#easychuan-audio');
  const preview = view.querySelector('#easychuan-preview');
  const fileOptions = view.querySelector('#easychuan-file-options');
  const textOptions = view.querySelector('#easychuan-text-options');
  const mediaOptions = view.querySelector('#easychuan-media-options');
  const screenHelp = view.querySelector('#easychuan-screen-help');
  const videoHelp = view.querySelector('#easychuan-video-help');
  const screenAudio = view.querySelector('#easychuan-screen-audio');
  const videoAudio = view.querySelector('#easychuan-video-audio');
  const generate = view.querySelector('#easychuan-generate');
  const stop = view.querySelector('#easychuan-stop');
  const message = view.querySelector('#easychuan-status');
  const share = view.querySelector('#easychuan-share');
  const urlInput = view.querySelector('#easychuan-url');
  const qrImage = view.querySelector('#easychuan-qr');
  const code = view.querySelector('#easychuan-code');
  const routeLabel = view.querySelector('#easychuan-route');
  let mode = modes.querySelector('input:checked').value;
  const tabs = Object.fromEntries(['text', 'file', 'screen', 'video'].map(kind => [
    kind, { pending: null, published: null, status: '', audio: false },
  ]));
  const selectedFiles = [];
  let editing = true;

  function render() {
    const screen = mode === 'screen';
    const writing = mode === 'text';
    const file = mode === 'file';
    const media = !file && !writing;
    const { pending, published, status } = tabs[mode];
    const progress = published?.progress;
    const link = published?.link;
    const stream = published?.stream || pending?.stream || null;
    if (preview.srcObject !== stream) preview.srcObject = stream;
    preview.hidden = !stream;
    fileOptions.hidden = !file;
    textOptions.hidden = !writing;
    mediaOptions.hidden = !media;
    screenHelp.hidden = screenAudio.hidden = !screen;
    videoHelp.hidden = videoAudio.hidden = screen;
    audio.checked = tabs[mode].audio;
    audio.disabled = Boolean(pending || published);
    choose.disabled = input.disabled = Boolean(tabs.file.pending);
    text.readOnly = !editing || Boolean(tabs.text.pending);

    const unchangedFiles = selectedFiles.length === published?.files?.length
      && selectedFiles.every((item, index) => item === published.files[index]);
    generate.textContent = t(media ? (screen ? '共享屏幕' : '开启摄像头') : '发送');
    generate.hidden = (writing && !editing) || (media && Boolean(published));
    generate.disabled = Boolean(pending) || (writing && (!editing || !text.value.trim()))
      || (file && (!selectedFiles.length || unchangedFiles)) || (media && Boolean(published));
    stop.textContent = t('结束分享');
    stop.hidden = writing || !(pending || published);
    message.textContent = status ? t(status)
      : progress ? t(messages[progress.phase], { file: progress.filename, percent: progress.percent })
      : published?.expiry ? t('文本已发送，有效期至 {date}', { date: new Intl.DateTimeFormat(view.ownerDocument.documentElement.lang, {
        dateStyle: 'short', timeStyle: 'short',
      }).format(published.expiry) })
      : t(published?.status || '');
    routeLabel.hidden = !progress;
    routeLabel.textContent = progress ? t(routes[progress.route]) : '';
    share.hidden = !link;
    urlInput.value = link?.url || '';
    code.textContent = link?.code || '';
    if (link) {
      if (qrImage.getAttribute('src') !== link.qr) qrImage.src = link.qr;
    } else {
      qrImage.removeAttribute('src');
    }
    renderFiles();
  }

  function renderFiles() {
    fileList.hidden = !selectedFiles.length;
    const count = selectedFiles.length;
    const summary = count ? t('已选文件数：{count}', { count }) : t('未选择文件');
    if (fileCount.textContent !== summary) fileCount.textContent = summary;
    const markup = selectedFiles.map((file, index) =>
      `<li><span>${esc(file.name)}</span><button type="button" class="text-button" data-remove-file="${index}" aria-label="${esc(t('移除 {file}', { file: file.name }))}"${input.disabled ? ' disabled' : ''}>${esc(t('移除'))}</button></li>`
    ).join('');
    if (fileList.innerHTML !== markup) fileList.innerHTML = markup;
  }

  function end(session) {
    session?.stop?.();
    session?.stream?.getTracks().forEach(track => track.stop());
  }

  modes.addEventListener('change', event => {
    mode = event.target.value;
    render();
  });
  audio.addEventListener('change', () => { tabs[mode].audio = audio.checked; });
  choose.addEventListener('click', () => input.click());
  input.addEventListener('change', () => {
    const added = [...input.files];
    input.value = '';
    tabs.file.status = '';
    for (const file of added) {
      if (selectedFiles.some(item => item.name === file.name && item.size === file.size
          && item.lastModified === file.lastModified && item.type === file.type)) continue;
      if (selectedFiles.length === maxFiles) {
        tabs.file.status = '轻松传单次最多选择 10 个文件';
        break;
      }
      selectedFiles.push(file);
    }
    render();
  });
  fileList.addEventListener('click', event => {
    const button = event.target.closest('[data-remove-file]');
    if (!button || input.disabled) return;
    selectedFiles.splice(Number(button.dataset.removeFile), 1);
    tabs.file.status = '';
    render();
    const buttons = fileList.querySelectorAll('[data-remove-file]');
    (buttons[Math.min(Number(button.dataset.removeFile), buttons.length - 1)] || choose).focus();
  });
  function requestEdit() {
    if (!editing && !tabs.text.pending && !editDialog.open) {
      editDialog.returnValue = '';
      editDialog.showModal();
    }
  }
  text.addEventListener('click', requestEdit);
  text.addEventListener('keydown', event => {
    if (!editing && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault();
      requestEdit();
    }
  });
  editDialog.addEventListener('close', () => {
    if (editDialog.returnValue !== 'edit') return;
    editing = true;
    tabs.text.status = '';
    render();
    text.focus();
  });
  text.addEventListener('input', () => {
    tabs.text.status = '';
    render();
  });
  stop.addEventListener('click', () => {
    const state = tabs[mode];
    const { pending, published } = state;
    state.pending = state.published = null;
    end(pending);
    end(published);
    state.status = '分享已结束';
    render();
  });

  view.querySelector('.easychuan-panel').addEventListener('submit', async event => {
    event.preventDefault();
    if (generate.disabled) return;
    const kind = mode;
    const state = tabs[kind];
    const current = {};
    const media = kind === 'screen' || kind === 'video';
    state.pending = current;
    state.status = media ? '正在等待选择或授权…' : '正在连接轻松传…';
    render();

    function fail(error) {
      if (state.pending === current) {
        state.pending = null;
      } else if (state.published === current) {
        state.published = null;
      } else {
        return;
      }
      end(current);
      if (!state.pending) state.status = String(error?.message ?? error);
      if (mode === kind) render();
    }

    function showLink({ code, url }, status) {
      if (state.pending !== current) return;
      const qr = qrcode(0, 'M');
      qr.addData(url);
      qr.make();
      current.link = { code, url, qr: qr.createDataURL(4, 16) };
      current.status = status;
      const previous = state.published;
      state.pending = null;
      state.published = current;
      state.status = '';
      if (kind === 'text') editing = false;
      end(previous);
      if (mode === kind) render();
    }

    const callbacks = {
      onProgress: next => {
        if (state.pending !== current && state.published !== current) return;
        const progress = current.progress;
        if (progress && next && next.phase === progress.phase && next.route === progress.route
            && next.filename === progress.filename && next.percent === progress.percent) return;
        current.progress = next;
        if (mode === kind && state.published === current) render();
      },
      onFailure: fail,
    };

    try {
      if (kind === 'text') {
        const { fetch_code: code, date_expire: date } = await registerText(text.value);
        if (state.pending !== current) return;
        const expiry = new Date(date);
        if (typeof code !== 'string' || !/^[a-z0-9]{1,32}$/i.test(code) || !Number.isFinite(expiry.getTime())) {
          throw new Error('轻松传返回了无效的取件信息');
        }
        current.expiry = expiry;
        showLink({ code, url: `https://easychuan.cn/r/${code}?t=t` }, '');
        return;
      }
      if (kind === 'file') {
        if (typeof RTCPeerConnection === 'undefined') throw new Error('当前系统 WebView 不支持 WebRTC 文件传输');
        const files = current.files = [...selectedFiles];
        const { startEasyChuanSender } = await import('./easychuan.js');
        if (state.pending !== current) return;
        const registration = await register(files.map(file => ({ filename: file.name, filesize: file.size })));
        if (state.pending !== current) return;
        current.stop = startEasyChuanSender({
          registration,
          files,
          ...callbacks,
          onReady: link => showLink(link, '等待接收方下载'),
        });
        return;
      }
      // Invoke the capture picker while the original submit still has user activation.
      const stream = await captureMedia(kind, state.audio);
      if (state.pending !== current) {
        stream.getTracks().forEach(track => track.stop());
        return;
      }
      current.stream = stream;
      const videoTrack = stream.getVideoTracks()[0];
      if (videoTrack?.readyState !== 'live') throw new Error('画面分享已停止');
      videoTrack.addEventListener('ended', () => fail('画面分享已停止'), { once: true });
      state.status = '正在连接轻松传…';
      if (mode === kind) render();
      const { startEasyChuanMedia } = await import('./easychuan.js');
      if (state.pending !== current) return;
      const registration = await registerMedia(kind, { stream_id: stream.id, has_audio: Boolean(stream.getAudioTracks().length) });
      if (state.pending !== current) return;
      current.stop = startEasyChuanMedia({
        kind,
        registration,
        stream,
        ...callbacks,
        onReady: link => showLink(link, '等待接收方观看'),
      });
    } catch (error) {
      fail(error);
    }
  });

  view.querySelector('#easychuan-copy').addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(urlInput.value);
      notify('取件链接已复制');
    } catch {
      urlInput.focus();
      urlInput.select();
      notify('无法复制，请手动选择并复制地址', true);
    }
  });

  render();
  return render;
}
