# 方序传文件（Fangxu File Transfer）

**简体中文** | [English](./README_EN.md)

[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-3446B7.svg)](./LICENSE)

> 方寸之间，传递有序。

免费、开源的**跨平台局域网文件传输工具**。在 Windows、macOS 或 Linux 电脑上共享目录，Android、iPhone、iPad 用浏览器扫码即可上传、下载文件，无需安装手机 App、注册账号或上传网盘。

适合电脑与手机互传照片、文档、电子书、视频和安装包，以及同一 Wi-Fi 下临时分享资料。

## 下载

前往 [GitHub Releases](https://github.com/goldenwind/fangxu-file-transfer/releases/latest) 下载，解压后运行，无需安装 Go。

| 系统                        | 下载文件                                    | 启动方式                                             |
| --------------------------- | ------------------------------------------- | ---------------------------------------------------- |
| Windows x86-64              | `Fangxu-File-Transfer-Windows-amd64.zip`  | 双击`双击运行.bat` 或 `Fangxu-File-Transfer.exe` |
| macOS Intel / Apple Silicon | `Fangxu-File-Transfer-macOS.zip`          | 双击`方序传文件.app`                               |
| Linux x86-64                | `Fangxu-File-Transfer-Linux-amd64.tar.gz` | 双击`双击运行.sh`                                  |
| Linux ARM64                 | `Fangxu-File-Transfer-Linux-arm64.tar.gz` | 双击`双击运行.sh`                                  |

Windows 首次运行请允许防火墙访问“专用网络”；macOS 首次被拦截时可右键 App 选择“打开”；Linux 文件管理器可能需要启用“将可执行文本文件作为程序运行”。

## 快速上手

1. 在电脑上启动程序，浏览器会自动打开控制页。
2. 选择并应用共享目录，默认使用当前用户的 `Downloads`。
3. 手机与电脑连接同一 Wi-Fi；多人网络中先开启“访问令牌保护”。
4. 手机扫码，用系统浏览器打开页面，点击文件下载，或选择多个文件上传到电脑。
5. 传完后在电脑页面点击“停止传输服务”，确认关闭后再关页面。

```text
电脑共享目录 <──同一 Wi-Fi / HTTP──> 手机或平板浏览器
```

## 界面预览

### 电脑端共享设置

选择共享目录，查看服务状态和访问地址，让手机或平板扫描二维码连接。

![电脑端共享设置：共享目录、访问令牌保护、服务状态与连接二维码](./assets/screenshots/desktop-settings.png)

### 文件上传与下载

通过浏览器上传文件，按类型和格式筛选、搜索与排序，也可勾选多个文件批量下载。

![文件浏览界面：上传、搜索、类型与格式筛选、排序和批量下载](./assets/screenshots/file-browser.png)

## 主要功能

- **双向直传**：任意文件类型，不经过第三方服务器；上传同名文件自动追加序号，不覆盖原文件。
- **目录共享**：扫描子目录，支持中文文件名、搜索、两级类型筛选、文件数量和时间 / 名称排序。
- **多文件上传**：每次最多 100 个文件、总大小上限 10 GB；失败时自动清理临时文件。下载支持 HTTP 断点续传。
- **电脑端管理**：输入目录或使用系统文件夹选择器，切换后立即刷新；显示访问地址、二维码与服务状态。
- **随用随停**：自动选择空闲端口，单实例运行；再次双击打开已有服务页面。
- **批量下载**：勾选多个文件，或“全选本页”选择当前搜索、分类筛选后可见的文件；批量逐个下载原文件；浏览器提示时需允许下载多个文件。切换筛选保留已选文件，可用“取消选择”清空。
- **访问令牌**：可限制只有持完整链接或新二维码的设备访问；网页不提供删除或重命名。

## ☕ 支持与关注

方序传文件会一直保持免费、开源。如果它帮你省下了时间，欢迎请作者喝杯咖啡、给项目一个 Star，或分享给需要在电脑与手机间传文件的朋友。

<table>
  <tr>
    <td align="center" width="33%"><strong>支付宝赞赏</strong><br><br><img src="https://cdn.ip21.cn/img/common/alipay-qrcode.jpg" width="160" alt="支付宝赞赏二维码"></td>
    <td align="center" width="33%"><strong>微信赞赏</strong><br><br><img src="https://cdn.ip21.cn/img/common/wechatpay-qrcode.jpg" width="160" alt="微信赞赏二维码"></td>
    <td align="center" width="33%"><strong>关注「一灯 AI」</strong><br><br><img src="https://cdn.ip21.cn/img/common/wechat-pub.png" width="160" alt="一灯 AI 微信公众号二维码"></td>
  </tr>
</table>

## 使用边界

- 仅适合可信局域网中的临时传输，不提供异地传输、自动同步或备份。
- **访问令牌保护默认关闭**，每次启动生成新令牌并恢复关闭。未开启时，能连接服务的设备可以浏览、下载和上传。
- 令牌限制访问，但不加密传输；服务使用 HTTP，不建议在公共 Wi-Fi 中传输敏感文件。
- 共享目录及非隐藏子目录会开放给访问者，以 `.` 开头的文件和目录隐藏。建议使用专用目录；上传文件也会出现在列表中。
- 文件能否打开或安装取决于接收设备的格式支持与系统限制。

## 常见问题

- **多个访问地址**：优先显示默认路由使用的私有 IPv4 地址，其次是 Wi-Fi、有线网卡地址；过滤 VPN、常见虚拟网卡、自分配及保留地址。排序是本机网络信息推断，不能保证绕过防火墙或路由器设备隔离。
- **手机打不开**：确认同一 Wi-Fi，检查 VPN / 代理、防火墙和路由器 AP / 客户端隔离设置。
- **微信内无法下载**：点击右上角“···”并选择“在浏览器打开”；iPhone 使用 Safari，Android 使用系统浏览器。
- **链接泄露**：停止并重启服务，开启访问令牌保护后分享新链接。只重启而不开启保护无法限制访问。

## 源码运行与构建

需要 Go 1.22+：

```bash
git clone https://github.com/goldenwind/fangxu-file-transfer.git
cd fangxu-file-transfer
go run . --dir "/需要分享的目录"
```

| 参数                      | 说明                                  |
| ------------------------- | ------------------------------------- |
| `--dir 路径`            | 共享目录，默认当前用户的`Downloads` |
| `--listen 0.0.0.0:9000` | 固定端口，默认自动选择空闲端口        |
| `--no-open`             | 不自动打开电脑浏览器                  |

```bash
go test ./...
go build -o fangxu-file-transfer .
./build-packages.sh
```

打包脚本在 `dist` 生成 Windows、macOS 和 Linux 分发包；完整打包需在 macOS 上运行，使用 `lipo`、`sips`、`codesign` 和 `ditto`。

## 其他传文件场景与方法建议

按设备组合选择方案；`↔` 表示双向互传。

| 平台 ↔ 平台                   | 浏览器方案                                                | 安装软件方案                                                                                   |
| ------------------------------ | --------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Windows ↔ Android             | **方序传文件**：电脑启动，手机浏览器上传 / 下载     | [Quick Share](https://support.google.com/android/answer/13801258?hl=zh-Hans)、数据线、LocalSend |
| Windows ↔ iPhone / iPad       | **方序传文件**：电脑启动，Safari 上传 / 下载        | LocalSend；长期共享用 SMB                                                                      |
| Windows ↔ macOS / Linux       | **方序传文件**：任一端启动，另一端浏览器上传 / 下载 | LocalSend；长期共享用 SMB / NAS                                                                |
| macOS ↔ Android               | **方序传文件**：Mac 启动，手机浏览器上传 / 下载     | LocalSend；长期共享用 SMB                                                                      |
| macOS ↔  iPhone / iPad       | **方序传文件**：Mac 启动，Safari 上传 / 下载        | [AirDrop（隔空投送）](https://support.apple.com/zh-cn/119857)、LocalSend                        |
| macOS ↔ Linux                 | **方序传文件**：任一端启动，另一端浏览器上传 / 下载 | LocalSend；长期共享用 SMB / NAS                                                                |
| Linux ↔ Android               | **方序传文件**：电脑启动，手机浏览器上传 / 下载     | 数据线、LocalSend                                                                              |
| Linux ↔ iPhone / iPad         | **方序传文件**：电脑启动，Safari 上传 / 下载        | LocalSend；长期共享用 SMB                                                                      |
| Android ↔ Android             | Android 文件管理器内置“无线传文件“                      | Quick Share、LocalSend                                                                         |
| Android ↔ iPhone / iPad       | Android 文件管理器内置“无线传文件“                      | LocalSend                                                                                      |
| iPhone / iPad ↔ iPhone / iPad | -                                                         | [AirDrop（隔空投送）](https://support.apple.com/zh-cn/119857)、LocalSend                        |

**同一可信局域网中，接收端只想用浏览器时，推荐本项目的 HTTP 方案：[下载方序传文件](https://github.com/goldenwind/fangxu-file-transfer/releases/latest)。**

电脑共享目录，其他设备通过浏览器直接上传、下载；传完即可停止服务。

[LocalSend](https://localsend.org/) 需在参与设备上均安装客户端，支持跨平台局域网加密传输。

数据线可访问的内容取决于手机系统。

异地传输可用网盘、邮件或聊天工具，具体容量、保存期限和压缩限制取决于服务。

如果目标设备是 Kindle，或需要发送到 Kindle 云端书库，请参阅[方序传书](https://github.com/goldenwind/fangxu-kindle-transfer/blob/main/README.md)，其中整理了浏览器下载、Send to Kindle、移动端分享和邮件传书方法。

## 开源许可

采用 [MIT 许可证](./LICENSE)。欢迎提交 [Issue](https://github.com/goldenwind/fangxu-file-transfer/issues)、功能建议和 Pull Request。
