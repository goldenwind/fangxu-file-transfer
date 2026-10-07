import { defineConfig } from 'vite';

export default defineConfig({
  clearScreen: false,
  server: { host: '127.0.0.1', port: 1438, strictPort: true, watch: { ignored: ['**/src-tauri/**', '**/build/**'] } },
  build: { outDir: 'desktop-dist', target: process.env.TAURI_ENV_PLATFORM === 'windows' ? 'chrome105' : 'safari15.4' },
});
