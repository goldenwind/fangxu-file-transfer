# 桌面客户端开发

桌面壳层参考 `fangxu-desktop-app-starter`，采用 Tauri 2、Rust、Vite 和原生 JavaScript。没有会员、账号、支付或额度限制。Go 继续负责局域网文件浏览、上传和下载。

## 运行

安装 Node.js 22.12+、Rust stable、Go 1.22+，以及 [Tauri 对应平台的构建依赖](https://v2.tauri.app/start/prerequisites/)。

```sh
npm ci
npm run desktop:dev
```

`npm run dev` 只启动网页开发服务器，无法控制本机服务。打开客户端时默认自动启动服务，使用已保存的共享目录或默认 Downloads；可在设置中关闭。设置中的“打开客户端时启动服务”仅在打开本应用时生效，不代表系统开机自启动。

```sh
npm run check
npm run desktop:build
```

原生安装包在 `src-tauri/target/release/bundle/` 中。macOS 生成 `.app` / `.dmg`，Windows 可生成 NSIS `.exe` / MSI，Linux 可生成 `.deb` / `.AppImage` / `.rpm`。可以用 `--bundles` 指定，例如：

```sh
npm run desktop:build -- --bundles app,dmg
npm run desktop:build -- --bundles nsis
npm run desktop:build -- --bundles deb,appimage
```

macOS DMG 打开后显示中英文拖拽安装引导：左侧为应用，右侧为指向 `/Applications` 的快捷方式，中间箭头提示将应用拖入「应用程序」。窗口尺寸、图标位置和背景由 `src-tauri/tauri.conf.json` 的 `bundle.macOS.dmg` 配置；660 × 460 的窗口为 660 × 400 的背景预留标题栏和路径栏空间，避免出现滚动条。背景设计源文件为 `packaging/macos/dmg-background.svg`，打包使用同目录的 PNG，无需在 CI 安装字体或图片转换工具。修改 SVG 后可使用 `rsvg-convert packaging/macos/dmg-background.svg -o packaging/macos/dmg-background.png` 更新 PNG。GitHub Actions 设置 `TAURI_BUNDLER_DMG_IGNORE_CI=true`，确保 macOS 构建也写入 Finder 背景和图标布局。

Windows x64 构建还可单独生成免安装 ZIP。在 Windows x64 上运行：

```sh
npm run desktop:build -- --no-bundle
python scripts/collect-release.py --platform Windows-x64 --portable-only
```

输出为 `release-assets/Fangxu-File-Transfer-<版本>-Windows-x64-portable.zip`，包含同目录的「方序传文件.exe」、`fangxu-transfer-service.exe`、中英文使用说明和许可证。主程序使用已有软件图标；用户完整解压后双击主程序即可启动，无需安装本软件。ZIP 使用 `build.rs` 生成的 x86_64 MSVC 传输组件，打包前检查必需文件，避免生成缺少组件的分发包。

免安装版依赖系统已安装的 [WebView2 Runtime](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution)。若缺少，可从微软安装 Evergreen Runtime，或使用本软件的安装版。配置仍写入下面的用户配置目录，与安装版共用，不随解压文件夹移动。

桌面版支持 macOS 12.3+、Windows 10 / 11（WebView2）和安装了 WebKitGTK 4.1 的 Linux。各平台安装包在对应系统上构建。CI 为 macOS Intel / Apple Silicon、Windows x64、Linux x64 / ARM64 安装包配置了测试与 artifact 上传，Windows 同时提供免安装 ZIP。推送与应用版本一致的 `v*` 标签（例如 `v1.1.2`）后，全部平台构建通过才发布 GitHub Release，安装版与免安装版均附带 SHA256 校验文件；手动运行仅构建产物，除非所选 ref 本身是版本标签。签名、公证需发布者提供证书。

图标的唯一设计源文件为 `assets/fangxu-file-transfer-app-icon.svg`，采用软件界面的柔和蓝色主色（`--accent: #5269c7`）作为背景、浅蓝白（`--accent-soft: #f0f3ff`）作为图形颜色，保留文件传到手机的标识。运行 `npm run desktop:icons`，生成 1024px PNG 和 `src-tauri/icons/` 中各平台所需的 PNG、ICNS 与 ICO；兼容旧路径的 `assets/fangxu-file-transfer-logo.png` 同步生成相同内容。桌面界面、网页 Logo 与 favicon、macOS 应用与 Dock、DMG、Windows 主程序、NSIS 安装与卸载程序及免安装版共用此设计。界面按原比例显示完整图标，不放大裁切。桌面打包前自动重新生成图标，避免设计稿与分发资源不一致。

客户端配色与浏览器传文件页面一致。右上角提供 GitHub 链接，使用系统浏览器打开；“中文 / EN”切换覆盖导航、状态、设置和帮助；语言选择保存在本机，重新打开后恢复，切换语言不会重置未保存的设置。

扫码后的浏览器文件页面不显示 GitHub 链接，语言切换位于 Logo 同行最右侧。

## 服务生命周期

Rust 在客户端原生初始化阶段立即启动内置 Go 子进程，不依赖页面加载或首次状态请求，参数为 `--desktop --config <应用配置目录>/settings.json`。开启自动启动时，Go 在接收首条页面指令之前启动传输服务。Go 通过每行一个 JSON 请求/响应的标准输入输出管道接受 `status`、`configure`、`start`、`stop`、`quit` 操作。网页不能通过 HTTP 管理此服务。

Go 的桌面模式在服务停止后保持待命，因此可以在同一窗口反复启动。目录和保护设置可以立即应用；运行中修改端口会被拒绝。每次启动服务重新生成访问令牌，开启令牌保护后应分享最新二维码。停止服务立即关闭连接并中断当前传输。

关闭主窗口或退出应用时 Rust 关闭子进程输入管道；Go 读到 EOF 会关闭监听并释放单实例锁。桌面进程崩溃也会关闭管道。重复打开客户端会聚焦已有窗口；如果旧版命令行服务已启动，客户端会提示先关闭原服务，不接管其他进程。

Go 子程序由 `src-tauri/build.rs` 按 Cargo 的目标平台自动编译，使用 Tauri 的 [`externalBin` sidecar 打包](https://v2.tauri.app/develop/sidecar/)方式。安装后用户无需安装 Go、Rust 或 Node.js。Windows 子进程隐藏控制台窗口。

## 配置与权限

设置位于 Tauri 的 `app_config_dir()/settings.json`：macOS 通常为 `~/Library/Application Support/com.fangxu.file-transfer/`，Windows 为 `%APPDATA%/com.fangxu.file-transfer/`，Linux 为 `$XDG_CONFIG_HOME/com.fangxu.file-transfer/` 或 `~/.config/com.fangxu.file-transfer/`。

默认目录为用户的 Downloads，端口为 0（自动选择），令牌保护默认关闭，自动启动默认开启；已有用户保存的自动启动偏好优先。目录不存在时仍可打开客户端并重新选择；不会替用户创建或分享新的目录。设置中的启动选项、端口和保护状态持久化，访问令牌不会写入设置文件。

所有本机操作由明确的 Rust 命令处理；前端不具备任意 shell 执行权限。远程文件页面由浏览器打开，不嵌入有本机命令权限的 WebView。HTTP 仅适用于可信局域网，令牌保护不提供传输加密。

## 原命令行版本

```sh
go run . --dir "/需要分享的目录"
./build-cli-packages.sh
```

CLI 与既有手机浏览器交互保持兼容。`build-cli-packages.sh` 保留原 Windows ZIP、macOS universal 包和 Linux 压缩包构建；`build-packages.sh` 现在调用桌面端原生构建。
