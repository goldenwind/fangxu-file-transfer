方序传文件 v1.1.0：macOS、Windows、Linux 原生桌面客户端。

- 在客户端设置共享目录、端口、访问令牌保护，并启动或停止服务。
- 默认打开客户端时立即启动服务，关闭窗口或退出应用时自动关闭服务。
- 客户端与浏览器文件页采用统一配色，支持中文 / English 切换。
- 手机扫码后直接在浏览器上传、下载、搜索和批量下载文件，无需手机 App 或账号。

- 新增产品反馈入口，在浏览器登录后提交问题、截图与建议；传输功能仍无需账号。

### 选择安装包

| 平台 | 安装包 |
| --- | --- |
| macOS Apple Silicon（M 系列芯片） | `macOS-arm64.dmg` |
| macOS Intel | `macOS-x64.dmg` |
| Windows 10 / 11 x64 | `Windows-x64.exe` |
| Linux x64 | `Linux-x64.deb` 或 `Linux-x64.AppImage` |
| Linux ARM64 | `Linux-arm64.deb` |

macOS 需 12.3 或更高版本。Linux deb 需 WebKitGTK 4.1（如 Ubuntu 22.04 或更新版本）。AppImage 运行前需添加执行权限。所有安装包已包含传输组件，无需安装 Go、Rust 或 Node.js。当前安装包未进行商业证书签名和 macOS 公证。`SHA256SUMS.txt` 可用于校验文件完整性。

English: native desktop clients for macOS (Intel / Apple Silicon), Windows x64, and Linux (x64 / ARM64), with bilingual controls, service auto-start and automatic shutdown on window close. Scan the QR code to upload and download in your phone browser.
