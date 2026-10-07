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

桌面版支持 macOS 12.3+、Windows 10 / 11（WebView2）和安装了 WebKitGTK 4.1 的 Linux。各平台安装包在对应系统上构建。CI 为 macOS Intel / Apple Silicon、Windows x64、Linux x64 / ARM64 安装包配置了测试与 artifact 上传。推送与应用版本一致的 `v*` 标签（例如 `v1.1.0`）后，全部平台构建通过才发布 GitHub Release，附带 SHA256 校验文件；手动运行仅构建产物，除非所选 ref 本身是版本标签。签名、公证需发布者提供证书。

桌面应用图标使用 `assets/fangxu-file-transfer-app-icon.png`，主体放大并居中，保留少量透明边距。更新此文件后运行 `npm run desktop:icons`，生成 `src-tauri/icons/` 中各平台所需的 PNG、ICNS 与 ICO，再构建安装包。macOS 应用、Dock 和 DMG 使用同一套图标；界面内的 Logo 继续使用原始素材。

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
