import { invoke, isTauri } from '@tauri-apps/api/core';
import { getVersion } from '@tauri-apps/api/app';
import { currentMonitor, getCurrentWindow, PhysicalPosition, PhysicalSize } from '@tauri-apps/api/window';
import packageInfo from '../package.json';
import logo from '../assets/fangxu-file-transfer-app-icon.png';
import baiduIcon from '../assets/cloud-drive/baidu.png';
import quarkIcon from '../assets/cloud-drive/quark.png';
import alipanIcon from '../assets/cloud-drive/alipan.png';
import weiyunIcon from '../assets/cloud-drive/weiyun.png';
import xunleiIcon from '../assets/cloud-drive/xunlei.png';
import tianyiIcon from '../assets/cloud-drive/tianyi.png';
import mobileIcon from '../assets/cloud-drive/mobile.png';
import pan123Icon from '../assets/cloud-drive/123pan.png';
import pan115Icon from '../assets/cloud-drive/115pan.png';
import wechatAppreciationQr from '../assets/donation/wechat-appreciation.png';
import alipayQr from '../assets/donation/alipay-qrcode.jpg';
import wechatPublicQr from '../assets/donation/wechat-pub.png';
import { escapeHTML as esc, readSettings } from './helpers.js';
import { initialLanguage, languageStorageKey, translations, translate, translateError } from './i18n.js';
import { setupEasyChuanView } from './easychuan-view.js';
import { relayServices, setupRelayView } from './relay-view.js';
import './styles.css';

const icons = {
  transfer: '<path d="M4 7h15m-4-4 4 4-4 4M20 17H5m4-4-4 4 4 4"/>',
  internet: '<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>',
  relay: '<path d="M3 8l9-5 9 5v8l-9 5-9-5Zm0 0 9 5 9-5m-9 5v8M7.5 5.5l9 5"/>',
  cloud: '<path d="M7 18H6a4 4 0 0 1-.6-8A6 6 0 0 1 17 8.5a4.8 4.8 0 0 1 1 9.5h-1M12 21V12m-3 3 3-3 3 3"/>',
  wechat: '<path d="M14 16a6.5 6.5 0 1 0-2-12.5C7.6 3 4 5.8 4 9c0 1.4.6 2.6 1.6 3.6L5 16l3.7-1.5c.8.3 1.7.5 2.6.5m3.2 4.5a5.5 5.5 0 1 1 6.4-4.1l1 3.1-3-1.1c-1.2.8-2.7 1-4.4.8M8.5 8.5h.1m3.9 0h.1m2.4 5.2h.1m3.2 0h.1"/>',
  settings: '<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/>',
  folder: '<path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10H3Z"/>',
  text: '<path d="M4 9V6h16v3M12 6v12m-3 0h6"/>',
  file: '<path d="M6 3h8l4 4v14H6zM14 3v5h4M9 12h6m-6 4h6"/>',
  screen: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M12 16v4m-4 0h8"/>',
  video: '<rect x="3" y="6" width="12" height="12" rx="2"/><path d="m15 10 6-3v10l-6-3"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  play: '<path d="m9 5 11 7-11 7Z"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V4H4v12h4"/>',
  heart: '<path d="M20.8 5.7a5 5 0 0 0-7.1 0L12 7.4l-1.7-1.7a5 5 0 0 0-7.1 7.1L12 21l8.8-8.2a5 5 0 0 0 0-7.1Z"/>',
  feedback: '<path d="M21 11a8 8 0 0 1-8 8H7l-4 3V11a8 8 0 0 1 8-8h2a8 8 0 0 1 8 8Z"/><path d="M8 9h8M8 13h5"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10v1"/>',
  shield: '<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6Z"/><path d="m8 12 3 3 5-6"/>',
  wifi: '<path d="M2 8a16 16 0 0 1 20 0M5 12a11 11 0 0 1 14 0M9 16a5 5 0 0 1 6 0m-3 4h.01"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>',
  moon: '<path d="M20.7 15.3A9 9 0 0 1 8.7 3.3 9 9 0 1 0 20.7 15.3Z"/>',
};
const icon = (name, cls = '') => `<svg class="icon ${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name]}</svg>`;
const githubUrl = 'https://github.com/goldenwind/fangxu-file-transfer';
const githubLogo = '<svg class="github-logo" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.65 7.65 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg>';
let status = null;
let busy = false;
let polling = false;
let revision = 0;
let selectedAddress = 0;
let connectionsKey = '';
let view = 'transfer';
let lastPollError = '';
let toastTimer;
let toastMessage;
let bootError = '';
let currentVersion = packageInfo.version;
let updateState = 'idle';
let updateResult = null;
let updateError = '';
let openingUpdate = false;
let openingFeedback = false;
let language = initialLanguage({ getItem: key => localStorage.getItem(key) }, navigator.language || 'zh');
const themeStorageKey = 'fangxu-theme';
const systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
let theme = 'system';
try {
  const savedTheme = localStorage.getItem(themeStorageKey);
  if (savedTheme === 'light' || savedTheme === 'dark') theme = savedTheme;
} catch { /* Use the system theme without storage. */ }

function applyTheme() {
  document.documentElement.dataset.theme = theme === 'system' ? (systemTheme.matches ? 'dark' : 'light') : theme;
}

applyTheme();

const t = (key, values) => translate(language, key, values);
const errorText = (error) => translateError(language, error);
const desktop = isTauri();
const macDesktop = desktop && navigator.platform.startsWith('Mac');
const cloudProviders = [
  ['baidu', '百度网盘', baiduIcon],
  ['quark', '夸克网盘', quarkIcon],
  ['alipan', '阿里云盘', alipanIcon],
  ['weiyun', '腾讯微云', weiyunIcon],
  ['xunlei', '迅雷云盘', xunleiIcon],
  ['tianyi', '天翼云盘', tianyiIcon],
  ['mobile', '移动云盘', mobileIcon],
  ['123pan', '123云盘', pan123Icon],
  ['115pan', '115网盘', pan115Icon],
];
const appWindow = desktop ? getCurrentWindow() : null;
const wechatOpenLink = `<a id="wechat-open" ${macDesktop ? '' : 'class="button primary"'} href="https://filehelper.weixin.qq.com/" target="_blank" rel="noopener noreferrer">${macDesktop ? '在独立窗口打开' : '打开微信文件传输助手'}</a>`;

document.querySelector('#app').innerHTML = `
  <aside class="sidebar${macDesktop ? ' sidebar-mac' : ''}" data-window-drag>
    <a class="brand" href="#" aria-label="方序快传首页"><span class="brand-logo"><img src="${logo}" alt=""/></span><span>方序<span class="brand-sub">快传</span></span></a>
    <div class="local-badge">${icon('shield')}本机直传 · 免费开源</div>
    <nav aria-label="主导航">
      <button class="nav-item active" data-view="transfer">${icon('transfer')}局域网互传<span class="nav-mark"></span></button>
      <button class="nav-item" data-view="easychuan">${icon('internet')}公网在线传<span class="nav-mark"></span></button>
      <button class="nav-item" data-view="relay">${icon('relay')}大文件中转<span class="nav-mark"></span></button>
      <button class="nav-item" data-view="cloud">${icon('cloud')}网盘发送<span class="nav-mark"></span></button>
      <button class="nav-item" data-view="wechat">${icon('wechat')}微信网页传输<span class="nav-mark"></span></button>
    </nav>
    <div class="sidebar-bottom"><button class="nav-item" data-view="donate" type="button">${icon('heart')}支持作者<span class="nav-mark"></span></button><a id="feedback-link" class="nav-item" href="https://api.ip21.cn/products/10/feedback" target="_blank" rel="noopener noreferrer">${icon('feedback')}产品反馈</a><button class="nav-item" data-view="about">${icon('info')}关于与帮助</button><span class="version">方序快传 <span id="app-version"></span></span></div>
  </aside>
  ${appWindow ? '<div class="window-drag-edge" data-window-drag aria-hidden="true"></div>' : ''}
  <div class="workspace">
    <main>
      <div class="top-actions" data-window-drag>
        <span class="theme-control">
          ${icon('sun', 'theme-icon theme-icon-light')}${icon('moon', 'theme-icon theme-icon-dark')}
          <select id="theme" class="theme-select" aria-label="外观主题"><option value="system">跟随系统</option><option value="light">浅色</option><option value="dark">深色</option></select>
        </span>
        <a id="github-link" class="github-link" data-open-github href="${githubUrl}" target="_blank" rel="noopener noreferrer" aria-label="GitHub 开源项目">${githubLogo} GitHub ${icon('arrow')}</a>
        <div class="language-switch" role="group" aria-label="语言 / Language"><button class="language-button" type="button" data-language="zh" aria-pressed="true">中文</button><button class="language-button" type="button" data-language="en" aria-pressed="false">EN</button></div>
      </div>
      <div id="boot" class="boot" role="status">正在读取本机设置并检查共享目录…</div>
      <section id="transfer-view" class="view" hidden>
        <div class="page-heading transfer-heading">
          <h1>局域网互传</h1>
          <p>让要传输文件的设备连接<strong>同一 Wi-Fi 或局域网</strong>。</p>
        </div>
        <article class="panel connection-panel">
          <div class="connection-header">
            <div class="connection-title"><h2>连接设备</h2><span id="status-badge" class="status-badge">服务未启动</span></div>
            <button id="toggle-service" class="button primary" type="button">${icon('play')}启动服务</button>
          </div>
          <div class="connection-content">
            <figure class="scan-area">
              <div class="qr-frame">
                <img id="connection-qr" alt="设备连接二维码" hidden/>
                <div id="qr-placeholder" aria-hidden="true">${icon('wifi')}</div>
              </div>
              <figcaption id="scan-caption" hidden>用手机相机扫码</figcaption>
            </figure>
            <div>
              <div id="connection-address" hidden>
                <h3>其他设备访问地址</h3>
                <p class="helper">在另一台设备的浏览器中打开此链接。</p>
                <code id="connection-url" class="address-box" tabindex="0"></code>
                <div class="connection-actions">
                  <button id="copy-address" class="button primary" type="button">${icon('copy')}复制链接</button>
                  <label id="network-picker" hidden><span>网络地址</span><select id="network-address"></select></label>
                </div>
              </div>
              <div id="connection-empty" role="status"><h3 id="connection-empty-title"></h3><p id="connection-empty-description" class="helper"></p></div>
              <div id="connection-footer" class="connection-footer" hidden>
                <button id="open-browser" class="text-button" type="button">在本机浏览器查看文件 ${icon('arrow')}</button>
              </div>
            </div>
          </div>
        </article>
        <form id="directory-form" class="panel directory-panel">
          <div class="directory-header">
            <h2>${icon('folder')}<label for="directory">共享目录</label></h2>
            <button id="open-directory" class="text-button" type="button">在文件管理器中打开 ${icon('arrow')}</button>
          </div>
          <div class="path-input"><input id="directory" name="directory" autocomplete="off" spellcheck="false" aria-describedby="directory-help"/><button id="choose-directory" type="button" class="button secondary">选择文件夹</button></div>
          <p id="directory-help" class="helper">上传和下载均使用此目录。选择文件夹或输入路径后按回车，立即生效。</p>
        </form>
        <section id="transfer-settings" class="panel transfer-settings" aria-labelledby="transfer-settings-title">
          <h2 id="transfer-settings-title">${icon('settings')}传输设置</h2>
          <form id="settings-form">
            <div class="transfer-options">
              <div class="settings-section"><div class="port-row"><label class="field-label" for="port">监听端口</label><input id="port" name="port" type="number" min="0" max="65535" step="1" required aria-describedby="port-help"/></div><p id="port-help" class="helper">0 表示自动分配。停止服务后可修改。</p></div>
              <label class="settings-section switch-row"><span id="protection-label" class="field-label">访问令牌保护</span><input id="protected" type="checkbox" role="switch" aria-labelledby="protection-label" aria-describedby="protection-help"/><span class="switch" aria-hidden="true"></span><span id="protection-help" class="helper">开启后，其他设备需要完整链接或二维码才能访问文件。</span></label>
              <label class="settings-section switch-row"><span id="auto-start-label" class="field-label">打开客户端时启动服务</span><input id="auto-start" type="checkbox" role="switch" checked aria-labelledby="auto-start-label" aria-describedby="auto-start-help"/><span class="switch" aria-hidden="true"></span><span id="auto-start-help" class="helper">启动时自动共享当前目录。</span></label>
            </div>
            <div class="settings-footer"><span id="settings-note" class="helper">HTTP 传输适用于可信局域网。令牌保护不加密文件内容。</span><button id="save-settings" type="submit" class="button primary" disabled>保存设置</button></div>
          </form>
        </section>
      </section>
      <section id="easychuan-view" class="view" hidden>
        <div class="page-heading transfer-heading">
          <h1>公网在线传</h1>
          <p><strong>跨网络传输</strong>，适合远程发送文本、文件，或实时共享屏幕和摄像头。</p>
          <p>接收方用浏览器打开链接。传文件或共享画面时，<strong>发送端需保持在线。</strong></p>
        </div>
        <form class="panel easychuan-panel">
          <fieldset class="easychuan-modes" aria-label="传输内容">
            <label><input type="radio" name="easychuan-mode" value="text" checked/><span class="easychuan-mode">${icon('text')}文本</span></label>
            <label><input type="radio" name="easychuan-mode" value="file"/><span class="easychuan-mode">${icon('file')}文件</span></label>
            <label><input type="radio" name="easychuan-mode" value="screen"/><span class="easychuan-mode">${icon('screen')}投屏</span></label>
            <label><input type="radio" name="easychuan-mode" value="video"/><span class="easychuan-mode">${icon('video')}摄像头</span></label>
          </fieldset>
          <div id="easychuan-file-options" role="group" aria-labelledby="easychuan-file-label" hidden>
            <div id="easychuan-file-label" class="field-label">选择要发送的文件</div>
            <div class="easychuan-file-picker">
              <button id="easychuan-choose" class="button secondary" type="button">${icon('folder')}选择文件</button>
              <span id="easychuan-file-count" role="status"></span>
            </div>
            <input id="easychuan-files" type="file" multiple hidden/>
            <ul id="easychuan-file-list" aria-label="已选文件" hidden></ul>
            <p class="helper">生成链接时会向轻松传发送所选文件的名称和大小。接收期间请保持客户端在线。</p>
          </div>
          <div id="easychuan-text-options">
            <label class="field-label" for="easychuan-text">要发送的文本</label>
            <textarea id="easychuan-text" rows="6" maxlength="50000"></textarea>
            <p class="helper">文本会暂存在轻松传，持有取件链接的人可以在有效期内查看。</p>
          </div>
          <div id="easychuan-media-options" hidden>
            <p id="easychuan-screen-help" class="helper">选择要分享的屏幕或窗口。观看期间请保持客户端在线。</p>
            <p id="easychuan-video-help" class="helper">开启摄像头分享实时画面。观看期间请保持客户端在线。</p>
            <label class="easychuan-audio"><input id="easychuan-audio" type="checkbox"/><span id="easychuan-screen-audio">同时分享系统声音（视设备支持情况）</span><span id="easychuan-video-audio">同时开启麦克风</span></label>
            <video id="easychuan-preview" autoplay muted playsinline aria-label="本地画面预览" hidden></video>
          </div>
          <div class="easychuan-actions">
            <button id="easychuan-generate" type="submit" class="button primary" disabled>发送</button>
            <button id="easychuan-stop" type="button" class="button secondary" hidden>结束分享</button>
          </div>
          <p id="easychuan-status" class="helper" role="status"></p>
          <p id="easychuan-route" class="helper" hidden></p>
          <div id="easychuan-share" class="easychuan-share" hidden>
            <img id="easychuan-qr" width="132" height="132" alt="" aria-label="轻松传取件二维码"/>
            <div class="easychuan-share-details">
              <p class="easychuan-code"><span>取件码</span> <strong id="easychuan-code"></strong></p>
              <label class="field-label" for="easychuan-url">取件链接</label>
              <div class="path-input"><input id="easychuan-url" type="url" readonly/><button id="easychuan-copy" class="button secondary" type="button">${icon('copy')}复制链接</button></div>
            </div>
          </div>
        </form>
        <dialog id="easychuan-edit-dialog" aria-labelledby="easychuan-edit-title" aria-describedby="easychuan-edit-description">
          <form method="dialog">
            <h2 id="easychuan-edit-title">编辑已发送的文本？</h2>
            <p id="easychuan-edit-description">编辑后再次发送会重新生成链接。</p>
            <div class="dialog-actions">
              <button class="button secondary" value="cancel">取消</button>
              <button class="button primary" value="edit" autofocus>继续编辑</button>
            </div>
          </form>
        </dialog>
      </section>
      <section id="relay-view" class="view" hidden>
        <div class="page-heading transfer-heading">
          <h1>大文件中转</h1>
          <p><strong>收件人可稍后下载</strong>，适合分享大视频、压缩包等文件。</p>
          <p><strong>免登录上传</strong>后即可分享取件链接，有效期和大小上限因服务而异。</p>
        </div>
        <p id="relay-desktop-only" class="panel" hidden>请在桌面客户端中使用大文件中转。</p>
        <form class="panel relay-panel">
          <div>
            <button id="relay-choose" type="button" class="button secondary">${icon('folder')}添加文件</button>
            <ul id="relay-files" aria-label="已选文件" hidden></ul>
          </div>
          <fieldset id="relay-providers">
            <legend>选择中转服务</legend>
            ${Object.entries(relayServices).map(([id, { name, detail, retention }]) => `<label><input type="checkbox" value="${id}" ${id === 'kiwi' ? 'checked' : ''}/><span><strong>${name}</strong><small><span>${detail}</span> · <span>${retention}</span></small></span></label>`).join('')}
          </fieldset>
          <div class="relay-actions">
            <button id="relay-submit" type="submit" class="button primary" disabled>上传到所选服务</button>
            <button id="relay-cancel" type="button" class="button stop-button" hidden>取消全部上传</button>
          </div>
          <p id="relay-status" class="helper" role="status"></p>
          <ul id="relay-results" aria-label="中转结果" hidden></ul>
        </form>
      </section>
      <section id="cloud-view" class="view" hidden>
        <div class="page-heading transfer-heading">
          <h1>网盘发送</h1>
          <p><strong>长期保存、反复分享</strong>的文件，适合放在自己常用的网盘。</p>
          <p><strong>使用已有网盘账号</strong>在官方页面上传、管理文件和生成分享链接。</p>
        </div>
        ${desktop ? `
          <div class="cloud-toolbar">
            <fieldset id="cloud-providers" class="easychuan-modes" aria-label="选择网盘">
              ${cloudProviders.map(([id, name, image], index) => `
                <label>
                  <input type="radio" name="cloud-provider" value="${id}" ${index === 0 ? 'checked' : ''}/>
                  <span><img src="${image}" width="16" height="16" alt=""/>${name}</span>
                </label>
              `).join('')}
            </fieldset>
            ${macDesktop ? '' : '<button type="button" class="button primary" id="cloud-open">在应用中打开</button>'}
            <button type="button" class="text-button" id="cloud-browser">在浏览器打开 ↗</button>
          </div>
          <p id="cloud-error" class="helper" role="status" hidden></p>
          ${macDesktop ? '<div id="cloud-webview" aria-label="网盘发送"></div>' : ''}
        ` : '<p class="panel">请在桌面客户端中使用网盘发送。</p>'}
      </section>
      <section id="wechat-view" class="view" hidden>
        <div class="page-heading transfer-heading">
          <h1>微信网页传输</h1>
          <p><strong>手机和电脑临时互传</strong>文字、照片或文件，适合把内容发给自己。</p>
          <p><strong>微信扫码登录</strong>，用微信文件传输助手直接收发。${macDesktop ? ` ${wechatOpenLink}` : ''}</p>
        </div>
        ${macDesktop ? '<div id="wechat-webview" aria-label="微信网页传输"></div>' : `<div class="panel">${wechatOpenLink}</div>`}
      </section>
      <section id="donate-view" class="view" hidden>
        <div class="page-heading"><div><h1>支持作者</h1></div></div>
        <header class="panel donate-thanks">
          <span class="donate-heart">${icon('heart')}</span>
          <div>
            <h2>给方序快传点一颗 Star</h2>
            <p>也许它让你少找了一次数据线，也许它帮你赶在截止前把文件发了出去。对我来说，一颗 Star 就是一句「这个工具对我有用」。每次收到这样的回应，我就多一点动力把它继续做好。</p>
            <div class="donate-star-actions">
              <a class="button primary" data-open-github href="${githubUrl}" target="_blank" rel="noopener noreferrer">${githubLogo}去 GitHub 点 Star</a>
              <p class="donate-star-hint">登录 GitHub 后，点击项目页右上角的 Star。</p>
            </div>
          </div>
        </header>
        <section class="donate-support" aria-labelledby="donate-support-title">
          <h2 id="donate-support-title">愿意请我喝杯咖啡吗？</h2>
          <p>如果方序快传帮到了你，欢迎扫码随心赞赏，支持我继续维护和打磨这个免费开源的小工具。</p>
          <div class="donate-methods">
            <figure class="panel donate-method">
              <figcaption><strong>微信赞赏</strong><span>用微信扫一扫</span></figcaption>
              <img class="donate-qr" src="${wechatAppreciationQr}" width="432" height="432" alt="微信赞赏二维码"/>
            </figure>
            <figure class="panel donate-method">
              <figcaption><strong>支付宝赞赏</strong><span>用支付宝扫一扫</span></figcaption>
              <img class="donate-qr" src="${alipayQr}" width="301" height="300" alt="支付宝赞赏二维码"/>
            </figure>
          </div>
        </section>
        <article class="panel donate-gift" aria-labelledby="donate-gift-title">
          <div>
            <p class="donate-gift-label">赞赏回礼</p>
            <h2 id="donate-gift-title">任意一款方序软件产品，获赠 1 个月高级会员</h2>
            <p class="donate-claim">任意金额均可领取。赞赏后，扫码关注「一灯 AI」公众号，留言告诉我你选了哪款产品。</p>
            <a class="button secondary" id="product-catalog-link" href="https://www.ip21.cn/works/" target="_blank" rel="noopener noreferrer">浏览全部产品 ${icon('arrow')}</a>
          </div>
          <figure><img class="donate-qr" src="${wechatPublicQr}" width="400" height="400" alt="一灯 AI 公众号二维码"/><figcaption>一灯 AI 公众号</figcaption></figure>
        </article>
      </section>
      <section id="about-view" class="view" hidden>
        <div class="page-heading"><div><h1>方寸之间，传递有序。</h1><p id="about-version"></p></div></div>
        <article class="panel update-panel" aria-labelledby="update-heading">
          <div class="panel-heading"><div><h2 id="update-heading">版本更新</h2><p id="current-version"></p></div><button id="check-update" type="button" class="button secondary"></button></div>
          <p id="update-status" role="status" aria-live="polite"></p>
          <div id="update-details" hidden><h3 id="update-title"></h3><p id="update-notes"></p><button id="download-update" type="button" class="button primary">下载新版本 ↗</button></div>
        </article>
        <article class="panel about-panel"><h2>从电脑到手机，只需三步</h2><ol><li>让电脑与接收设备连接同一 Wi-Fi 或局域网。</li><li>在「局域网互传」中选择共享目录。如果服务未启动，点击「启动服务」。</li><li>接收设备扫描二维码，使用系统浏览器上传或下载文件。</li></ol><h2>遇到连接问题？</h2><p>确认设备处于同一网络，检查 VPN、代理、防火墙及路由器的设备隔离设置。Windows 防火墙询问时，请允许专用网络访问。微信内无法下载时，请选择“在浏览器打开”。</p><h2>传完之后</h2><p>关闭主窗口后，传输服务会在托盘继续运行。点击“停止服务”或在托盘菜单选择“退出”，即可结束共享并中断当前传输。每次重新启动服务都会生成新的访问令牌。</p><h2>局域网文件与隐私</h2><p>文件不会上传到第三方服务器。隐藏目录与以点开头的文件不会显示；同名上传自动追加序号。单次最多上传 100 个文件，总大小不超过 10 GB。</p></article></section>
    </main>
    <footer><span id="footer-status">本机服务未启动</span><span>macOS · Windows · Linux</span></footer>
  </div>
  <div id="toast" role="status" aria-live="polite" hidden></div>
  <dialog id="stop-dialog" aria-labelledby="stop-title" aria-describedby="stop-description">
    <h2 id="stop-title">停止传输服务？</h2>
    <p id="stop-description">正在进行的上传和下载会中断。你可以随时重新启动服务。</p>
    <div class="dialog-actions">
      <button id="cancel-stop" class="button secondary">继续传输</button>
      <button id="confirm-stop" class="button danger" autofocus>停止服务</button>
    </div>
  </dialog>
`;

const $ = (selector) => document.querySelector(selector);
const renderEasyChuan = setupEasyChuanView($('#easychuan-view'), {
  register: files => invoke('register_easychuan_files', { files }),
  registerText: content => invoke('register_easychuan_text', { content }),
  registerMedia: (kind, stream) => invoke('register_easychuan_media', { kind, stream }),
  translate: t,
  notify,
});

const renderRelay = setupRelayView($('#relay-view'), { invoke, desktop, t, notify, language: () => language });

// Capture static template text once. Translating text nodes preserves SVG icons,
// native controls and event handlers. Runtime status text is translated in render().
const staticText = [];
const walker = document.createTreeWalker($('#app'), NodeFilter.SHOW_TEXT);
while (walker.nextNode()) {
  const node = walker.currentNode;
  const key = node.textContent.trim();
  if (Object.hasOwn(translations, key)) staticText.push({ node, key, before: node.textContent.match(/^\s*/)[0], after: node.textContent.match(/\s*$/)[0] });
}
const staticAttributes = ['aria-label', 'alt'].flatMap(attribute =>
  [...document.querySelectorAll(`[${attribute}]`)]
    .map(element => ({ element, attribute, key: element.getAttribute(attribute) }))
    .filter(({ key }) => Object.hasOwn(translations, key)));

function settingsChanged() {
  return status && ($('#port').value !== String(status.settings.port)
    || $('#protected').checked !== status.settings.protected
    || $('#auto-start').checked !== status.settings.autoStart);
}

function renderSettingsState() {
  const changed = settingsChanged();
  $('#settings-note').textContent = t(changed ? '有未保存的修改' : 'HTTP 传输适用于可信局域网。令牌保护不加密文件内容。');
  $('#save-settings').disabled = busy || !changed;
}

function renderBoot() {
  if (status) return;
  if (bootError) $('#boot').textContent = t('{error}。修复后可重新打开客户端。', { error: errorText(bootError) });
  else if (desktop) $('#boot').textContent = t('正在读取本机设置并检查共享目录…');
  else $('#boot').innerHTML = `<h2>${esc(t('请在桌面客户端中打开'))}</h2><p>${esc(t('此页面需要客户端提供本机服务控制。在项目目录运行'))} <code>npm run desktop:dev</code> ${esc(t('启动客户端。'))}</p>`;
}

function setLanguage(next) {
  language = next === 'en' ? 'en' : 'zh';
  document.documentElement.lang = language === 'en' ? 'en' : 'zh-CN';
  document.title = t('方序快传');
  for (const { node, key, before, after } of staticText) node.textContent = before + t(key) + after;
  for (const { element, attribute, key } of staticAttributes) element.setAttribute(attribute, t(key));
  for (const button of document.querySelectorAll('[data-language]')) {
    const active = button.dataset.language === language;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
  }
  if (toastMessage && !$('#toast').hidden) $('#toast').textContent = toastMessage.error ? errorText(toastMessage.message) : t(toastMessage.message);
  renderSettingsState();
  renderBoot();
  renderUpdates();
  renderEasyChuan();
  renderRelay();
  if (cloudErrorMessage) showCloudError(cloudErrorMessage);
  render();
  fitCloudToolbar();
  try { localStorage.setItem(languageStorageKey, language); } catch { /* Keep switching available without storage. */ }
  if (desktop) invoke('set_tray_language', { language }).catch(error => console.error(error));
}

const themeSelect = $('#theme');
themeSelect.value = theme;
themeSelect.addEventListener('change', () => {
  theme = themeSelect.value;
  applyTheme();
  try { localStorage.setItem(themeStorageKey, theme); } catch { /* Keep switching available without storage. */ }
});
systemTheme.addEventListener('change', () => {
  if (theme === 'system') applyTheme();
});

for (const button of document.querySelectorAll('[data-language]')) button.addEventListener('click', () => setLanguage(button.dataset.language));
for (const link of document.querySelectorAll('[data-open-github]')) link.addEventListener('click', async (event) => {
  if (!desktop) return;
  event.preventDefault();
  try { await invoke('open_github'); }
  catch (error) { notify(error, true); }
});
$('#product-catalog-link').addEventListener('click', async event => {
  if (!desktop) return;
  event.preventDefault();
  try { await invoke('open_product_catalog'); }
  catch (error) { notify(error, true); }
});
if (macDesktop) {
  let controlsHidden = false;
  let updateControls = Promise.resolve();
  const syncControls = () => {
    const hidden = Boolean(document.querySelector('dialog[open]'));
    if (hidden === controlsHidden) return;
    controlsHidden = hidden;
    updateControls = updateControls.then(() => invoke('set_native_window_controls_hidden', { hidden }))
      .catch(error => console.error(error));
  };
  const observer = new MutationObserver(syncControls);
  for (const dialog of document.querySelectorAll('dialog')) {
    observer.observe(dialog, { attributes: true, attributeFilter: ['open'] });
  }
}
if (appWindow) {
  const runWindowAction = action => action().catch(error => notify(error, true));
  for (const region of document.querySelectorAll('[data-window-drag], .page-heading')) {
    region.addEventListener('mousedown', event => {
      if (event.button !== 0 || event.target.closest('a, button, input, select, textarea, label, summary, [contenteditable]')) return;
      if (region.classList.contains('page-heading') && event.target !== region) return;
      event.preventDefault();
      event.stopPropagation();
      if (event.detail === 1) runWindowAction(() => appWindow.startDragging());
      if (event.detail === 2) runWindowAction(() => appWindow.toggleMaximize());
    });
  }
}

const embeddedHosts = { cloud: $('#cloud-webview'), wechat: $('#wechat-webview') };
const embeddedCommands = { cloud: 'set_cloud_drive_view', wechat: 'set_wechat_view' };
const cloudError = $('#cloud-error');
let cloudErrorMessage = '';
const selectedCloudProvider = () => $('#cloud-providers input:checked').value;
let embeddedView = null;
let appliedLayout = '';
let layoutPending = false;
let applyingLayout = false;

async function fitCloudToolbar() {
  if (!appWindow || view !== 'cloud') return;
  const toolbar = $('.cloud-toolbar');
  const providers = $('#cloud-providers');
  const buttons = toolbar.querySelectorAll('button');
  const totalWidth = elements => Array.from(elements).reduce((width, element) => width + element.getBoundingClientRect().width, 0);
  const rowWidth = totalWidth(providers.children) + (providers.children.length - 1) * parseFloat(getComputedStyle(providers).columnGap)
    + totalWidth(buttons) + buttons.length * parseFloat(getComputedStyle(toolbar).columnGap);
  if (rowWidth <= toolbar.clientWidth) return;
  const rootStyle = getComputedStyle(document.documentElement);
  const desktopInset = parseFloat(rootStyle.getPropertyValue('--desktop-sidebar'))
    + 2 * parseFloat(rootStyle.getPropertyValue('--desktop-main-padding'));
  const desiredWidth = Math.ceil((rowWidth + desktopInset) * window.devicePixelRatio);

  try {
    const [monitor, inner, outer, position, fullscreen, maximized] = await Promise.all([
      currentMonitor(), appWindow.innerSize(), appWindow.outerSize(), appWindow.outerPosition(),
      appWindow.isFullscreen(), appWindow.isMaximized(),
    ]);
    if (view !== 'cloud' || !monitor || fullscreen || maximized) return;
    const { position: workOrigin, size: workSize } = monitor.workArea;
    const borderWidth = outer.width - inner.width;
    const width = Math.min(desiredWidth, workSize.width - borderWidth);
    if (width <= inner.width) return;
    const x = Math.max(workOrigin.x, Math.min(position.x, workOrigin.x + workSize.width - width - borderWidth));
    if (x !== position.x) await appWindow.setPosition(new PhysicalPosition(x, position.y)).catch(console.error);
    if (view === 'cloud') await appWindow.setSize(new PhysicalSize(width, inner.height));
  } catch (error) {
    console.error(error);
  }
}

function showCloudError(message) {
  cloudErrorMessage = message;
  cloudError.textContent = message ? errorText(message) : '';
  cloudError.hidden = !message;
}

async function syncEmbeddedLayout() {
  layoutPending = true;
  if (applyingLayout) return;
  applyingLayout = true;
  while (layoutPending) {
    layoutPending = false;
    const next = embeddedHosts[view] ? view : null;
    const rect = next ? embeddedHosts[next].getBoundingClientRect() : null;
    const bounds = rect?.width > 0 && rect.height > 0
      ? { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
      : null;
    const args = next === 'cloud' ? { provider: selectedCloudProvider(), bounds } : { bounds };
    const layout = JSON.stringify([next, args]);
    try {
      if (embeddedView && (embeddedView !== next || !bounds)) {
        const hide = embeddedView === 'cloud' ? { provider: null, bounds: null } : { bounds: null };
        await invoke(embeddedCommands[embeddedView], hide);
        embeddedView = null;
        appliedLayout = '';
      }
      if (next && bounds && layout !== appliedLayout) {
        // A failed update can still have created a pane. Keep track of it so it can be hidden.
        embeddedView = next;
        await invoke(embeddedCommands[next], args);
        appliedLayout = layout;
      }
    } catch (error) {
      if (view === 'cloud') showCloudError(error);
      else if (view === 'wechat') notify(error, true);
    }
  }
  applyingLayout = false;
}

if (macDesktop) {
  const observer = new ResizeObserver(syncEmbeddedLayout);
  for (const host of Object.values(embeddedHosts)) observer.observe(host);
}
$('#cloud-providers')?.addEventListener('change', () => {
  showCloudError('');
  if (macDesktop) syncEmbeddedLayout();
});
for (const [id, external] of [['cloud-open', false], ['cloud-browser', true]]) {
  const button = $('#' + id);
  button?.addEventListener('click', async () => {
    showCloudError('');
    button.disabled = true;
    try { await invoke('open_cloud_drive', { provider: selectedCloudProvider(), language, external }); }
    catch (error) { showCloudError(error); }
    finally { button.disabled = false; }
  });
}

$('#wechat-open').addEventListener('click', async event => {
  if (!desktop) return;
  event.preventDefault();
  if (event.currentTarget.dataset.opening) return;
  const link = event.currentTarget;
  link.dataset.opening = 'true';
  try { await invoke('open_wechat_transfer', { language }); }
  catch (error) { notify(error, true); }
  finally { delete link.dataset.opening; }
});
$('#feedback-link').addEventListener('click', async (event) => {
  if (!desktop) return;
  event.preventDefault();
  if (openingFeedback) return;
  openingFeedback = true;
  try { await invoke('open_product_feedback', { language }); }
  catch (error) { notify(error, true); }
  finally { openingFeedback = false; }
});
function renderUpdates() {
  $('#app-version').textContent = `v${currentVersion}`;
  $('#about-version').textContent = t('方序快传 · v{version} · 免费开源', { version: currentVersion });
  $('#current-version').textContent = t('当前版本：v{version}', { version: currentVersion });
  $('#check-update').textContent = t(updateState === 'checking' ? '正在检查更新…' : '检查更新');
  $('#check-update').disabled = !desktop || updateState === 'checking' || openingUpdate;
  const available = updateState === 'done' && updateResult?.update_available;
  $('#update-details').hidden = !available;
  const message = !desktop ? '请在桌面客户端中检查更新。' : updateState === 'checking' ? '正在检查更新…'
    : updateState === 'error' ? '更新检查未完成，请重试。' : available ? (updateResult.force_update ? '当前版本需要升级，请下载并安装：' : '发现新版本：')
    : updateState === 'done' ? '当前已是最新版本。' : '手动检查是否有适用于此电脑的新版本。';
  $('#update-status').textContent = t(message)
    + (available ? ` v${updateResult.latest_version.replace(/^[vV]/, '')}` : '')
    + (updateState === 'error' ? ` ${errorText(updateError)}` : '');
  $('#update-status').classList.toggle('error', updateState === 'error' || Boolean(available && updateResult.force_update));
  $('#update-title').textContent = available ? updateResult.title : '';
  $('#update-title').hidden = !updateResult?.title;
  $('#update-notes').textContent = available ? updateResult.release_notes : '';
  $('#update-notes').hidden = !updateResult?.release_notes;
  $('#download-update').disabled = openingUpdate;
}

$('#check-update').addEventListener('click', async () => {
  if (!desktop || updateState === 'checking' || openingUpdate) return;
  updateState = 'checking'; updateResult = null; updateError = ''; renderUpdates();
  try {
    updateResult = await invoke('check_product_update');
    currentVersion = updateResult.current_version;
    updateState = 'done';
  } catch (error) {
    updateState = 'error'; updateError = String(error);
  } finally { renderUpdates(); }
});
$('#download-update').addEventListener('click', async () => {
  if (!desktop || openingUpdate || !updateResult?.update_available) return;
  openingUpdate = true; renderUpdates();
  try { await invoke('open_product_update'); }
  catch (error) { updateError = String(error); updateState = 'error'; }
  finally { openingUpdate = false; renderUpdates(); }
});
if (desktop) void getVersion().then(version => { currentVersion = version; renderUpdates(); }).catch(error => console.error(error));
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
  const previous = view;
  view = next;
  const viewReady = Boolean(status) || ['about', 'donate', 'relay', 'cloud', 'wechat'].includes(next) || (desktop && next === 'easychuan');
  $('#boot').hidden = viewReady;
  for (const element of document.querySelectorAll('.view')) element.hidden = !viewReady || element.id !== `${next}-view`;
  for (const button of document.querySelectorAll('.nav-item')) button.classList.toggle('active', button.dataset.view === next);
  if (next !== previous && (embeddedHosts[next] || embeddedHosts[previous])) {
    const main = $('main');
    main.classList.toggle('webview-inline', Boolean(embeddedHosts[next]));
    if (embeddedHosts[next]) main.scrollTop = 0;
    syncEmbeddedLayout();
  }
  if (next !== previous && next === 'cloud') fitCloudToolbar();
}

function renderSettingsForm() {
  $('#port').value = status.settings.port;
  $('#protected').checked = status.settings.protected;
  $('#auto-start').checked = status.settings.autoStart;
}

function renderConnectionAddress() {
  const address = status.addresses[selectedAddress];
  const qr = $('#connection-qr');
  if (qr.getAttribute('src') !== address.qrCode) qr.src = address.qrCode;
  const url = $('#connection-url');
  if (url.textContent !== address.url) {
    url.textContent = address.url;
    url.scrollLeft = 0;
  }
}

function renderConnections() {
  const { running, addresses } = status;
  const nextKey = JSON.stringify([running, addresses, language]);
  if (nextKey === connectionsKey) return;
  connectionsKey = nextKey;
  const ready = running && addresses.length > 0;
  $('#connection-qr').hidden = !ready;
  $('#scan-caption').hidden = !ready;
  $('#qr-placeholder').hidden = ready;
  $('#connection-address').hidden = !ready;
  $('#connection-empty').hidden = ready;
  $('#connection-footer').hidden = !running;
  if (!ready) {
    $('#connection-empty-title').textContent = t(running ? '还没有可用的局域网地址' : '等待开启连接');
    $('#connection-empty-description').textContent = t(running ? '请连接 Wi-Fi 或有线网络。连接后地址会自动刷新。' : '启动服务后，这里会生成连接地址和专属二维码。');
    return;
  }
  selectedAddress = Math.min(selectedAddress, addresses.length - 1);
  $('#network-picker').hidden = addresses.length < 2;
  const select = $('#network-address');
  select.replaceChildren(...addresses.map(({ url }, index) => new Option(new URL(url).host, index)));
  select.value = selectedAddress;
  renderConnectionAddress();
}

function render() {
  if (!status) return;
  $('#boot').hidden = true;
  const running = status.running;
  $('#status-badge').textContent = t(running ? '服务运行中' : '服务未启动');
  $('#status-badge').classList.toggle('running', running);
  $('#toggle-service').innerHTML = busy ? esc(t('正在处理…')) : `${icon(running ? 'stop' : 'play')}${esc(t(running ? '停止服务' : '启动服务'))}`;
  $('#toggle-service').className = `button ${running ? 'stop-button' : 'primary'}`;
  $('#footer-status').textContent = running ? t('服务运行中 · 端口 {port} · {protection}', { port: status.port, protection: t(status.settings.protected ? '令牌保护已开启' : '令牌保护未开启') }) : t('本机服务未启动');
  for (const button of document.querySelectorAll('main button:not([data-language], #easychuan-view button, #relay-view button)')) button.disabled = busy;
  renderSettingsState();
  renderUpdates();
  $('#port').disabled = busy || running;
  $('#directory').readOnly = busy;
  for (const input of document.querySelectorAll('#settings-form input:not(#port)')) input.disabled = busy;
  renderConnections();
  showView(view);
}

function applyStatus(next) {
  const previous = status;
  status = next;
  if (!previous) renderSettingsForm();
  if (next.settings.directory !== previous?.settings.directory) $('#directory').value = next.settings.directory;
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
$('#open-browser').addEventListener('click', () => operation('open_file_browser'));
$('#network-address').addEventListener('change', (event) => {
  selectedAddress = Number(event.target.value);
  renderConnectionAddress();
});
$('#copy-address').addEventListener('click', async () => {
  try { await navigator.clipboard.writeText(status.addresses[selectedAddress].url); notify('连接地址已复制'); }
  catch { notify('无法复制，请手动选择并复制地址', true); }
});
$('#open-directory').addEventListener('click', () => operation('open_shared_directory'));
$('#toggle-service').addEventListener('click', () => {
  if (status.running) { $('#stop-dialog').showModal(); return; }
  if (settingsChanged()) { notify('请先保存传输设置，再启动服务', true); $('#save-settings').focus(); return; }
  operation('start_service', undefined, '传输服务已启动');
});
$('#cancel-stop').addEventListener('click', () => $('#stop-dialog').close());
$('#confirm-stop').addEventListener('click', () => { $('#stop-dialog').close(); operation('stop_service', undefined, '传输服务已停止'); });

async function changeDirectory(path) {
  if (busy || !status) return;
  const directory = path.trim();
  if (!directory) notify('请选择或输入共享目录', true);
  else if (directory !== status.settings.directory) {
    await operation('save_settings', { settings: { ...status.settings, directory } }, '共享目录已更新');
  }
  $('#directory').value = status.settings.directory;
}

$('#directory-form').addEventListener('submit', (event) => {
  event.preventDefault();
  changeDirectory($('#directory').value);
});
$('#choose-directory').addEventListener('click', async () => {
  const path = await operation('choose_directory', { language });
  if (path) await changeDirectory(path);
});
$('#settings-form').addEventListener('input', renderSettingsState);
$('#settings-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  if (busy || !settingsChanged()) return;
  try {
    const settings = readSettings({ directory: status.settings.directory, port: $('#port').value, protected: $('#protected').checked, autoStart: $('#auto-start').checked });
    const result = await operation('save_settings', { settings }, '设置已保存');
    if (result) { renderSettingsForm(); renderSettingsState(); }
  } catch (error) { notify(error.message, true); }
});

async function poll() {
  if (!desktop || busy || polling || (document.hidden && status)) return;
  polling = true;
  const currentRevision = revision;
  try {
    const next = await invoke('service_status');
    if (next && currentRevision === revision) applyStatus(next);
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
