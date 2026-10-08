方序传文件 v1.1.2：macOS、Windows、Linux 原生桌面客户端。

- 在客户端设置共享目录、端口、访问令牌保护，并启动或停止服务。
- 默认打开客户端时立即启动服务，关闭窗口或退出应用时自动关闭服务。
- 初始窗口扩大至 1080 × 800，中英文关于与帮助页完整显示。
- 更新应用图标和 macOS DMG 安装界面。
- 客户端与浏览器文件页采用统一配色，支持中文 / English 切换。
- 手机扫码后直接在浏览器上传、下载、搜索和批量下载文件，无需手机 App 或账号。

- 产品反馈入口携带软件版本、操作系统、系统版本和 CPU 架构，在应用内反馈窗口登录后随问题、截图与建议提交；传输功能仍无需账号。
- 关于与帮助顶部提供检查更新，显示当前版本、新版本说明及下载入口。
- Windows 同时提供安装版和免安装 ZIP，免安装版完整解压后双击「方序传文件.exe」启动。

### 选择下载包

| 平台 | 安装包 |
| --- | --- |
| macOS Apple Silicon（M 系列芯片） | `macOS-arm64.dmg` |
| macOS Intel | `macOS-x64.dmg` |
| Windows 10 / 11 x64 | `Windows-x64.exe` |
| Windows 10 / 11 x64（免安装） | `Windows-x64-portable.zip` |
| Linux x64 | `Linux-x64.deb` 或 `Linux-x64.AppImage` |
| Linux ARM64 | `Linux-arm64.deb` |

macOS 需 12.3 或更高版本。Linux deb 需 WebKitGTK 4.1（如 Ubuntu 22.04 或更新版本）。AppImage 运行前需添加执行权限。所有安装包已包含传输组件，无需安装 Go、Rust 或 Node.js。当前安装包未进行商业证书签名和 macOS 公证。`SHA256SUMS.txt` 可用于校验文件完整性。

Windows 免安装版请完整解压，并保留主程序旁的 `fangxu-transfer-service.exe`。需要系统已安装 [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)；若缺少，可安装该组件或使用安装版。设置与安装版共用用户配置目录，不随解压文件夹移动。

English: native desktop clients for macOS (Intel / Apple Silicon), Windows x64, and Linux (x64 / ARM64), with in-app product feedback, update checks, a larger default window, refreshed icons and DMG layout, bilingual controls, service auto-start and automatic shutdown on window close. Windows also has a separate portable ZIP: extract fully and double-click 方序传文件.exe, keeping fangxu-transfer-service.exe beside it. Requires WebView2 Runtime; settings remain in the user configuration directory. Scan the QR code to upload and download in your phone browser.
