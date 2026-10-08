方序传文件 v1.1.4：假期照片批量传输。

- 手机浏览器新增「选择照片」入口、照片预览和上传队列，一次最多选择 5,000 个文件，上传中也可继续添加；当前列表中的重复选择自动跳过。
- 文件逐个上传，整批可超过 10 GB，单个文件需小于 10 GB。照片保持原文件与原格式，HEIC / HEIF 无预览时仍可传输。
- 每页显示 50 个预览，支持未完成、失败、已保存筛选和定位当前上传；显示已保存数量、整体进度、速度与预计剩余时间。
- 支持当前文件完成后暂停、继续上传和连接中断后重试。电脑确认保存后才计为成功；当前页面与服务进程内重试使用相同上传 ID，避免重复保存。
- 上传后自动刷新电脑已收到的文件列表，保留下载筛选和选择；同名文件继续自动追加序号，不覆盖已有照片。
- 中英文 README 增加假期照片备份步骤、传输限制与暂停恢复说明。

大批量传输请接上手机充电器，保持页面在前台，避免手机锁屏、电脑休眠或切换 Wi-Fi。队列只保留在当前页面；刷新或关闭后需重新选择，服务重启后不会保留重试回执。超过 5,000 个文件可传完后清空记录，再选下一批。未启用 JavaScript 时，整次表单上传仍需小于 10 GB。

验证：浏览器实际传输 5,000 个测试文件，核对全部文件内容，无重复文件和残留临时文件；覆盖上传中追加、暂停恢复、断网重试、中英文界面和分页。自动测试覆盖大容量队列、上传确认、令牌保护、并发重试和失败清理。

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

English: v1.1.4 improves holiday photo transfers from a phone browser. Select up to 5,000 files, preview 50 per page, add more during upload, filter unfinished or failed files, and check speed and estimated time remaining. Files upload individually in their original format with no total batch size limit; each file must be under 10 GB. Pause after the current file, continue, or retry unsuccessful uploads with save confirmations and retry IDs that prevent duplicate saves within the current page and service process. Received files refresh automatically. Keep the page in the foreground; reloading loses the queue and restarting the service clears retry receipts. Native packages are available for macOS (Intel / Apple Silicon), Windows x64 (installer and portable ZIP), and Linux (x64 / ARM64), with SHA256 checksums.
