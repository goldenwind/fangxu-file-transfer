import { invoke, isTauri } from '@tauri-apps/api/core';
import logo from '../assets/fangxu-file-transfer-logo.png';
import { escapeHTML as esc, readSettings, settingsEqual } from './helpers.js';
import { initialLanguage, languageStorageKey, translations, translate, translateError } from './i18n.js';
import './styles.css';

const icons = {
  transfer: '<path d="M4 7h15m-4-4 4 4-4 4M20 17H5m4-4-4 4 4 4"/>',
  settings: '<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/>',
  folder: '<path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10H3Z"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  play: '<path d="m9 5 11 7-11 7Z"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V4H4v12h4"/>',
  feedback: '<path d="M21 11a8 8 0 0 1-8 8H7l-4 3V11a8 8 0 0 1 8-8h2a8 8 0 0 1 8 8Z"/><path d="M8 9h8M8 13h5"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10v1"/>',
  shield: '<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6Z"/><path d="m8 12 3 3 5-6"/>',
  wifi: '<path d="M2 8a16 16 0 0 1 20 0M5 12a11 11 0 0 1 14 0M9 16a5 5 0 0 1 6 0m-3 4h.01"/>',
};
const icon = (name, cls = '') => `<svg class="icon ${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name]}</svg>`;
const githubLogo = '<svg class="github-logo" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.65 7.65 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg>';
let status = null;
let busy = false;
let polling = false;
let revision = 0;
let draft = null;
let selectedAddress = 0;
let connectionsKey = '';
let view = 'transfer';
let lastPollError = '';
let toastTimer;
let toastMessage;
let bootError = '';
let language = initialLanguage({ getItem: key => localStorage.getItem(key) }, navigator.language || 'zh');
const t = (key, values) => translate(language, key, values);
const errorText = (error) => translateError(language, error);
const desktop = isTauri();

document.querySelector('#app').innerHTML = `
  <aside class="sidebar">
    <a class="brand" href="#" aria-label="方序传文件首页"><span class="brand-logo"><img src="${logo}" alt=""/></span><span>方序<span class="brand-sub">传文件</span></span></a>
    <div class="sidebar-caption">工作空间</div>
    <nav aria-label="主导航">
      <button class="nav-item active" data-view="transfer">${icon('transfer')}文件传输<span class="nav-mark"></span></button>
      <button class="nav-item" data-view="settings">${icon('settings')}传输设置</button>
    </nav>
    <div class="sidebar-bottom"><div class="local-badge">${icon('shield')}本机直传 · 免费开源</div><a id="feedback-link" class="nav-item" href="https://api.ip21.cn/products/10/feedback" target="_blank" rel="noopener noreferrer">${icon('feedback')}产品反馈</a><button class="nav-item" data-view="about">${icon('info')}关于与帮助</button><span class="version">方序传文件 <span>v1.1.0</span></span></div>
  </aside>
  <div class="workspace">
    <main>
      <div class="top-actions"><a id="github-link" class="github-link" href="https://github.com/goldenwind/fangxu-file-transfer" target="_blank" rel="noopener noreferrer" aria-label="GitHub 开源项目">${githubLogo} GitHub ${icon('arrow')}</a><div class="language-switch" role="group" aria-label="语言 / Language"><button class="language-button" type="button" data-language="zh" aria-pressed="true">中文</button><button class="language-button" type="button" data-language="en" aria-pressed="false">EN</button></div></div>
      <div id="boot" class="boot" role="status">正在读取本机设置…</div>
      <section id="transfer-view" class="view" hidden>
        <div class="page-heading"><div><h1>让文件，轻松抵达。</h1><p>电脑与手机连接同一 Wi-Fi，即可上传和下载。</p></div></div>
        <div class="service-card"><div class="service-symbol">${icon('transfer')}</div><div class="service-copy"><div class="service-heading"><h2 id="service-title">准备好开始传输了吗？</h2><span id="status-badge" class="status-badge">服务未启动</span></div><p id="service-description">启动服务后，其他设备即可访问共享目录。</p></div><button id="toggle-service" class="button primary">${icon('play')}启动服务</button></div>
        <div class="transfer-grid">
          <article class="panel connection-panel"><div class="panel-heading"><div><h2>连接设备</h2><p>用手机系统相机或浏览器扫码</p></div>${icon('wifi')}</div><div id="connections"></div></article>
          <div class="right-column"><article class="panel directory-panel"><div class="panel-heading"><h2>共享目录</h2><button class="text-button" data-view="settings">修改 ${icon('arrow')}</button></div><div class="folder-visual">${icon('folder')}</div><strong id="directory-name">Downloads</strong><p id="directory-path" class="directory-path"></p><p class="helper">此目录及其非隐藏子目录中的文件可被下载。上传的文件也会保存在这里。</p><button id="open-directory" class="button secondary full">${icon('folder')}在文件管理器中打开</button></article><article class="privacy-note">${icon('shield')}<div><h3>文件在设备之间直接传递</h3><p>无需账号，无需网盘。退出客户端后，传输服务自动关闭。</p></div></article></div>
        </div>
        <div class="steps"><div><span>01</span><p>连接同一 Wi-Fi</p></div><div><span>02</span><p>启动服务并扫码</p></div><div><span>03</span><p>上传或下载文件</p></div></div>
      </section>
      <section id="settings-view" class="view" hidden>
        <div class="page-heading"><div><h1>传输设置</h1><p>选择分享的内容，按你的习惯开始传输。</p></div></div>
        <form id="settings-form" class="panel settings-panel">
          <div class="settings-section"><label class="field-label" for="directory">共享目录</label><p class="helper">请选择需要分享的文件夹，确认其中没有不希望公开的文件。</p><div class="path-input"><input id="directory" name="directory" autocomplete="off" spellcheck="false" required/><button id="choose-directory" type="button" class="button secondary">${icon('folder')}选择文件夹</button></div></div>
          <div class="settings-section"><label class="field-label" for="port">监听端口</label><div class="port-row"><input id="port" name="port" type="number" min="0" max="65535" step="1" required/><p class="helper">0 表示自动选择空闲端口。<br/>修改端口前需先停止服务。</p></div></div>
          <label class="settings-section switch-row"><span><span class="field-label">访问令牌保护</span><span class="helper">开启后，其他设备需要完整链接或二维码才能访问文件。</span></span><input id="protected" type="checkbox" role="switch"/><span class="switch" aria-hidden="true"></span></label>
          <label class="settings-section switch-row"><span><span class="field-label">打开客户端时启动服务</span><span class="helper">使用已保存的目录、端口与保护设置自动开始共享。</span></span><input id="auto-start" type="checkbox" role="switch" checked/><span class="switch" aria-hidden="true"></span></label>
          <div class="settings-footer"><span id="settings-note" class="helper">设置保存在这台电脑上。</span><button id="save-settings" type="submit" class="button primary">保存设置</button></div>
        </form>
        <p class="bottom-note">${icon('info')}服务使用 HTTP 传输，请在可信局域网中使用；令牌保护不加密文件内容。</p>
      </section>
      <section id="about-view" class="view" hidden><div class="page-heading"><div><h1>方寸之间，传递有序。</h1><p>方序传文件 · v1.1.0 · 免费开源</p></div></div><article class="panel about-panel"><h2>从电脑到手机，只需三步</h2><ol><li>让电脑与接收设备连接同一 Wi-Fi 或局域网。</li><li>在传输设置中选择共享目录，回到文件传输页面启动服务。</li><li>接收设备扫描二维码，使用系统浏览器上传或下载文件。</li></ol><h2>遇到连接问题？</h2><p>确认设备处于同一网络，检查 VPN、代理、防火墙及路由器的设备隔离设置。Windows 防火墙询问时，请允许专用网络访问。微信内无法下载时，请选择“在浏览器打开”。</p><h2>传完之后</h2><p>点击“停止服务”或退出客户端，其他设备将无法继续访问。停止会中断正在进行的传输。每次重新启动服务都会生成新的访问令牌。</p><h2>文件与隐私</h2><p>文件不会上传到第三方服务器。隐藏目录与以点开头的文件不会显示；同名上传自动追加序号。单次最多上传 100 个文件，总大小不超过 10 GB。</p></article></section>
    </main>
    <footer><span id="footer-status">本机服务未启动</span><span>macOS · Windows · Linux</span></footer>
  </div>
  <div id="toast" role="status" aria-live="polite" hidden></div>
  <dialog id="stop-dialog"><h2>停止传输服务？</h2><p>正在进行的上传和下载会中断。你可以随时重新启动服务。</p><div class="dialog-actions"><button id="cancel-stop" class="button secondary">继续传输</button><button id="confirm-stop" class="button danger">停止服务</button></div></dialog>
`;

const $ = (selector) => document.querySelector(selector);

// Capture static template text once. Translating text nodes preserves SVG icons,
// native controls and event handlers. Runtime status text is translated in render().
const staticText = [];
const walker = document.createTreeWalker($('#app'), NodeFilter.SHOW_TEXT);
while (walker.nextNode()) {
  const node = walker.currentNode;
  const key = node.textContent.trim();
  if (Object.hasOwn(translations, key)) staticText.push({ node, key, before: node.textContent.match(/^\s*/)[0], after: node.textContent.match(/\s*$/)[0] });
}
const staticAttributes = [...document.querySelectorAll('[aria-label]')]
  .filter(element => Object.hasOwn(translations, element.getAttribute('aria-label')))
  .map(element => ({ element, key: element.getAttribute('aria-label') }));

function renderSettingsNote() {
  $('#settings-note').textContent = t(draft && status && !settingsEqual(draft, status.settings) ? '有未保存的修改' : '设置保存在这台电脑上。');
}

function renderBoot() {
  if (status) return;
  if (bootError) $('#boot').textContent = t('{error}。修复后可重新打开客户端。', { error: errorText(bootError) });
  else if (desktop) $('#boot').textContent = t('正在读取本机设置…');
  else $('#boot').innerHTML = `<h2>${esc(t('请在桌面客户端中打开'))}</h2><p>${esc(t('此页面需要客户端提供本机服务控制。在项目目录运行'))} <code>npm run desktop:dev</code> ${esc(t('启动客户端。'))}</p>`;
}

function setLanguage(next) {
  language = next === 'en' ? 'en' : 'zh';
  document.documentElement.lang = language === 'en' ? 'en' : 'zh-CN';
  document.title = t('方序传文件');
  for (const { node, key, before, after } of staticText) node.textContent = before + t(key) + after;
  for (const { element, key } of staticAttributes) element.setAttribute('aria-label', t(key));
  for (const button of document.querySelectorAll('[data-language]')) {
    const active = button.dataset.language === language;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
  }
  if (toastMessage && !$('#toast').hidden) $('#toast').textContent = toastMessage.error ? errorText(toastMessage.message) : t(toastMessage.message);
  renderSettingsNote();
  renderBoot();
  render();
  try { localStorage.setItem(languageStorageKey, language); } catch { /* Keep switching available without storage. */ }
}

for (const button of document.querySelectorAll('[data-language]')) button.addEventListener('click', () => setLanguage(button.dataset.language));
$('#github-link').addEventListener('click', async (event) => {
  if (!desktop) return;
  event.preventDefault();
  try { await invoke('open_github'); }
  catch (error) { notify(error, true); }
});
$('#feedback-link').addEventListener('click', async (event) => {
  if (!desktop) return;
  event.preventDefault();
  try { await invoke('open_product_feedback'); }
  catch (error) { notify(error, true); }
});
setLanguage(language);

function notify(message, error = false) {
  toastMessage = { message, error };
  clearTimeout(toastTimer);
  $('#toast').textContent = error ? errorText(message) : t(message);
  $('#toast').classList.toggle('error', error);
  $('#toast').hidden = false;
  toastTimer = setTimeout(() => { $('#toast').hidden = true; }, error ? 7000 : 3500);
}

function showView(next) {
  view = next;
  for (const element of document.querySelectorAll('.view')) element.hidden = element.id !== `${next}-view` || !status;
  for (const button of document.querySelectorAll('.nav-item')) button.classList.toggle('active', button.dataset.view === next);
}

function formSettings() {
  return readSettings({ directory: $('#directory').value, port: $('#port').value, protected: $('#protected').checked, autoStart: $('#auto-start').checked });
}

function renderForm() {
  if (!draft) return;
  $('#directory').value = draft.directory;
  $('#port').value = draft.port;
  $('#protected').checked = draft.protected;
  $('#auto-start').checked = draft.autoStart;
}

function renderConnections() {
  const container = $('#connections');
  selectedAddress = Math.max(0, Math.min(selectedAddress, status.addresses.length - 1));
  const nextKey = JSON.stringify([status.running, status.addresses, status.settings.protected, selectedAddress, language]);
  // Keep the QR image, network selector and focused controls intact during polling.
  if (nextKey === connectionsKey) return;
  connectionsKey = nextKey;
  if (!status.running) {
    container.innerHTML = `<div class="qr-placeholder"><div class="qr-outline"><i></i><i></i><i></i><i></i>${icon('wifi')}</div><h3>${esc(t('等待开启连接'))}</h3><p>${esc(t('启动服务后，这里会生成连接地址和专属二维码。'))}</p></div>`;
    return;
  }
  if (!status.addresses.length) {
    container.innerHTML = `<div class="qr-placeholder"><div class="qr-outline">${icon('wifi')}</div><h3>${esc(t('还没有可用的局域网地址'))}</h3><p>${esc(t('请连接 Wi-Fi 或有线网络。连接后地址会自动刷新。'))}</p></div><button id="open-browser" class="button secondary full">${esc(t('在本机浏览器查看文件'))}</button>`;
  } else {
    const address = status.addresses[selectedAddress];
    container.innerHTML = `<div class="qr-display"><img src="${esc(address.qrCode)}" alt="${esc(t('设备连接二维码'))}"/><span class="qr-caption">${status.settings.protected ? icon('shield') + esc(t('令牌保护已开启')) : esc(t('扫一扫，连接这台电脑'))}</span></div>${status.addresses.length > 1 ? `<label class="address-label" for="network-address">${esc(t('可用连接地址'))}</label><select id="network-address">${status.addresses.map((item, index) => `<option value="${index}" ${index === selectedAddress ? 'selected' : ''}>${esc(new URL(item.url).host)}</option>`).join('')}</select>` : `<span class="address-label">${esc(t('局域网连接地址'))}</span>`}<div class="address-box"><code title="${esc(address.url)}">${esc(address.url)}</code><button id="copy-address" aria-label="${esc(t('复制连接地址'))}" title="${esc(t('复制连接地址'))}">${icon('copy')}</button></div><button id="open-browser" class="button secondary full">${esc(t('在本机浏览器查看文件'))} ${icon('arrow')}</button>`;
  }
  $('#open-browser')?.addEventListener('click', () => operation('open_file_browser'));
  $('#network-address')?.addEventListener('change', (event) => { selectedAddress = Number(event.target.value); renderConnections(); });
  $('#copy-address')?.addEventListener('click', async () => {
    try { await navigator.clipboard.writeText(status.addresses[selectedAddress].url); notify('连接地址已复制'); }
    catch { notify('无法复制，请手动选择并复制地址', true); }
  });
}

function render() {
  if (!status) return;
  $('#boot').hidden = true;
  const running = status.running;
  $('#status-badge').textContent = t(running ? '服务运行中' : '服务未启动');
  $('#status-badge').classList.toggle('running', running);
  $('#service-title').textContent = t(running ? '已准备好，等待设备连接' : '准备好开始传输了吗？');
  $('#service-description').textContent = running ? t('正在端口 {port} 上共享文件，扫码即可开始。', { port: status.port }) : t('启动服务后，其他设备即可访问共享目录。');
  $('#toggle-service').innerHTML = busy ? esc(t('正在处理…')) : `${icon(running ? 'stop' : 'play')}${esc(t(running ? '停止服务' : '启动服务'))}`;
  $('#toggle-service').className = `button ${running ? 'stop-button' : 'primary'}`;
  $('#directory-path').textContent = status.settings.directory;
  $('#directory-path').title = status.settings.directory;
  $('#directory-name').textContent = status.settings.directory.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || status.settings.directory;
  $('#footer-status').textContent = running ? t('服务运行中 · 端口 {port} · {protection}', { port: status.port, protection: t(status.settings.protected ? '令牌保护已开启' : '令牌保护未开启') }) : t('本机服务未启动');
  for (const button of document.querySelectorAll('main button:not([data-language])')) button.disabled = busy;
  $('#port').disabled = busy || running;
  for (const input of document.querySelectorAll('#settings-form input:not(#port)')) input.disabled = busy;
  renderConnections();
  $('#open-browser') && ($('#open-browser').disabled = busy);
  showView(view);
}

function applyStatus(next) {
  status = next;
  if (!draft) { draft = { ...status.settings }; renderForm(); }
  render();
  if (next.error && next.error !== lastPollError) notify(next.error, true);
  lastPollError = next.error || '';
}

async function operation(command, args, success) {
  if (busy) return;
  revision += 1;
  busy = true; render();
  try {
    const result = await invoke(command, args);
    if (result?.settings) applyStatus(result);
    if (success) notify(success);
    return result;
  } catch (error) { notify(String(error), true); }
  finally { busy = false; render(); }
}

for (const button of document.querySelectorAll('[data-view]')) button.addEventListener('click', () => showView(button.dataset.view));
$('.brand').addEventListener('click', (event) => { event.preventDefault(); showView('transfer'); });
$('#open-directory').addEventListener('click', () => operation('open_shared_directory'));
$('#toggle-service').addEventListener('click', () => {
  if (status.running) { $('#stop-dialog').showModal(); return; }
  if (!settingsEqual(draft, status.settings)) { notify('请先保存传输设置，再启动服务', true); showView('settings'); return; }
  operation('start_service', undefined, '传输服务已启动');
});
$('#cancel-stop').addEventListener('click', () => $('#stop-dialog').close());
$('#confirm-stop').addEventListener('click', () => { $('#stop-dialog').close(); operation('stop_service', undefined, '传输服务已停止'); });
$('#settings-form').addEventListener('input', () => {
  // Keep the raw draft while editing so polling never overwrites user input.
  draft = { directory: $('#directory').value, port: Number($('#port').value), protected: $('#protected').checked, autoStart: $('#auto-start').checked };
  renderSettingsNote();
});
$('#choose-directory').addEventListener('click', async () => {
  const path = await operation('choose_directory', { language });
  if (path) { $('#directory').value = path; $('#directory').dispatchEvent(new Event('input', { bubbles: true })); }
});
$('#settings-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  try {
    const settings = formSettings();
    const result = await operation('save_settings', { settings }, '设置已保存');
    if (result) { draft = { ...result.settings }; renderForm(); renderSettingsNote(); }
  } catch (error) { notify(error.message, true); }
});

async function poll() {
  if (!desktop || busy || polling || (document.hidden && status)) return;
  polling = true;
  const currentRevision = revision;
  try {
    const next = await invoke('service_status');
    if (currentRevision === revision) applyStatus(next);
  }
  catch (error) {
    if (currentRevision !== revision) return;
    const message = String(error);
    if (message !== lastPollError) notify(message, true);
    lastPollError = message;
    if (status) { status = { ...status, running: false, port: 0, addresses: [] }; render(); }
    else { bootError = message; renderBoot(); }
  } finally { polling = false; }
}

if (desktop) { poll(); setInterval(poll, 2500); }
else { renderBoot(); }
