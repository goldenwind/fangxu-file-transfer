方序快传 2.0.0 增加公网在线传、大文件中转、网盘发送和微信网页传输，保留同一局域网内的扫码互传。

- **公网在线传**：通过轻松传分享文字和文件，或实时共享屏幕与摄像头。收件人用浏览器打开取件链接，文件发送与实时共享期间请保持发送端在线。
- **大文件中转**：上传到选定的第三方服务，收件人可稍后取件。单文件上限与有效期依服务而定。
- **网盘与微信**：从侧栏访问各网盘的官方页面和微信文件传输助手，使用相应服务的账号。
- **局域网互传**：连接二维码、共享目录和传输设置集中显示，布局随窗口宽度调整。手机可扫码上传、下载和管理传输队列。
- **界面与体验**：更新品牌图标和彩色侧栏，增加浅色、深色和跟随系统主题，文件选择器跟随应用语言，新增 GitHub Star 支持入口。

### 选择下载包

| 平台 | 安装包 |
| --- | --- |
| macOS Apple Silicon（M 系列芯片） | `macOS-arm64.dmg` |
| macOS Intel | `macOS-x64.dmg` |
| Windows 10 / 11 x64 | `Windows-x64.exe` |
| Windows 10 / 11 x64（免安装） | `Windows-x64-portable.zip` |
| Linux x64 | `Linux-x64.deb` 或 `Linux-x64.AppImage` |
| Linux ARM64 | `Linux-arm64.deb` |

macOS 需 12.3 或更高版本。Linux deb 需 WebKitGTK 4.1（如 Ubuntu 22.04 或更新版本）。AppImage 运行前需添加执行权限。所有安装包已包含传输组件，无需安装 Go、Rust 或 Node.js。当前安装包未使用开发者证书签名或 macOS 公证。`SHA256SUMS.txt` 可用于校验文件完整性。

Windows 免安装版请完整解压，并保留主程序旁的 `fangxu-transfer-service.exe`。需要系统已安装 [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)；若缺少，可安装该组件或使用安装版。设置与安装版共用用户配置目录，不随解压文件夹移动。

English: Fangxu File Transfer 2.0.0 adds online text and file sharing, live screen and camera streaming, large-file relay, and shortcuts to official cloud drive and WeChat transfer services. LAN transfer now has a responsive layout with the QR code and share settings in one view. The desktop app also adds light and dark themes and a localized file picker.
