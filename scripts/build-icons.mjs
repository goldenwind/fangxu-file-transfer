import { execFileSync } from 'node:child_process';
import { copyFileSync, cpSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const output = mkdtempSync(join(tmpdir(), 'fangxu-file-icons-'));
const source = resolve(root, 'assets/fangxu-file-transfer-app-icon.svg');
const cli = resolve(root, 'node_modules/@tauri-apps/cli/tauri.js');
try {
  execFileSync(process.execPath, [cli, 'icon', source, '--output', join(output, 'platforms'), '--ios-color', '#5269c7'], { cwd: root, stdio: 'inherit' });
  execFileSync(process.execPath, [cli, 'icon', source, '--output', join(output, 'master'), '--png', '1024'], { cwd: root, stdio: 'inherit' });
  cpSync(join(output, 'platforms'), resolve(root, 'src-tauri/icons'), { recursive: true });
  for (const name of ['fangxu-file-transfer-app-icon.png', 'fangxu-file-transfer-logo.png']) {
    copyFileSync(join(output, 'master/1024x1024.png'), resolve(root, 'assets', name));
  }
} finally {
  rmSync(output, { recursive: true, force: true });
}
