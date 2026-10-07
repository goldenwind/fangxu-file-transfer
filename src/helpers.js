export function escapeHTML(value = '') {
  return String(value).replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
}

export function readSettings({ directory, port, protected: protectedValue, autoStart }) {
  const path = directory.trim();
  if (!path) throw new Error('请选择或输入共享目录');
  const rawPort = String(port).trim();
  if (!/^\d+$/.test(rawPort) || Number(rawPort) > 65535) throw new Error('端口必须为 0 到 65535，0 表示自动分配');
  return { directory: path, port: Number(rawPort), protected: Boolean(protectedValue), autoStart: Boolean(autoStart) };
}

export function settingsEqual(a, b) {
  return a.directory === b.directory && a.port === b.port && a.protected === b.protected && a.autoStart === b.autoStart;
}
