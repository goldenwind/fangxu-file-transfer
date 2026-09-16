# Fangxu File Transfer（方序传文件）

[简体中文](./README.md) | **English**

[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-3446B7.svg)](./LICENSE)

A free, open-source **cross-platform LAN file transfer tool**. Share a folder from a Windows, macOS or Linux computer. Scan a QR code on Android, iPhone or iPad to upload and download files in a browser, with no mobile app, account or cloud storage required.

Use it to transfer photos, documents, ebooks, videos and installers between your computer and phone, or temporarily share files over the same Wi-Fi network. The application interface is currently in Chinese.

## Download

Get a package from [GitHub Releases](https://github.com/goldenwind/fangxu-file-transfer/releases/latest), extract it and launch. Go is not required.

| System | Package | Launch |
| --- | --- | --- |
| Windows x86-64 | `Fangxu-File-Transfer-Windows-amd64.zip` | Double-click `双击运行.bat` or `Fangxu-File-Transfer.exe` |
| macOS Intel / Apple Silicon | `Fangxu-File-Transfer-macOS.zip` | Double-click `方序传文件.app` |
| Linux x86-64 | `Fangxu-File-Transfer-Linux-amd64.tar.gz` | Double-click `双击运行.sh` |
| Linux ARM64 | `Fangxu-File-Transfer-Linux-arm64.tar.gz` | Double-click `双击运行.sh` |

On Windows, allow firewall access on private networks. If macOS blocks the app on first launch, right-click it and choose Open. Your Linux file manager may require enabling execution of text scripts.

## Quick start

1. Launch the program on your computer. The control page opens automatically.
2. Select and apply a shared folder; the default is your user's `Downloads` folder.
3. Connect your phone and computer to the same Wi-Fi. On a network shared with others, enable access token protection (`访问令牌保护`) first.
4. Scan the QR code on your phone and open it in the system browser. Tap a file to download, or select multiple files to upload to your computer.
5. Click Stop transfer service (`停止传输服务`) on the computer page when finished. Wait for shutdown confirmation before closing the page.

```text
Computer folder <──same Wi-Fi / HTTP──> Phone or tablet browser
```

## Screenshots

### Desktop sharing settings

Select a shared folder, view the service status and LAN address, and scan the QR code from a phone or tablet to connect.

![Desktop sharing settings: shared folder, access token protection, service status and connection QR code](./assets/screenshots/desktop-settings.png)

### File uploads and downloads

Upload files in a browser, search and sort the list, filter by type and format, or select multiple files for batch download.

![File browser: uploads, search, type and format filters, sorting and batch downloads](./assets/screenshots/file-browser.png)

## Features

- **Two-way direct transfer:** any file type, without third-party servers. Uploads with duplicate names receive a numbered suffix instead of overwriting files.
- **Folder sharing:** subfolder scanning, Chinese filenames, search, two-level file type filters, file counts, and sorting by time or name.
- **Multiple uploads:** up to 100 files and 10 GB total per request; temporary files are cleaned up on failure. Downloads support HTTP resume.
- **Desktop controls:** enter a folder path or use the system folder picker; switching folders refreshes the list immediately. View LAN addresses, QR codes and service status.
- **Temporary sharing:** automatic free port selection and a single running instance. Launching again opens the existing service page.
- **Access tokens:** optionally restrict access to devices with the full link or new QR code. The web interface offers no deletion or renaming.

## Support & follow

Fangxu File Transfer will remain free and open source. If it saves you time, you can buy the author a coffee, give the project a Star, or share it with friends who need to transfer files between their computer and phone.

<table>
  <tr>
    <td align="center" width="33%"><strong>Alipay</strong><br><br><img src="https://cdn.ip21.cn/img/common/alipay-qrcode.jpg" width="160" alt="Alipay donation QR code"></td>
    <td align="center" width="33%"><strong>WeChat Pay</strong><br><br><img src="https://cdn.ip21.cn/img/common/wechatpay-qrcode.jpg" width="160" alt="WeChat Pay donation QR code"></td>
    <td align="center" width="33%"><strong>Follow Yideng AI (一灯 AI)</strong><br><br><img src="https://cdn.ip21.cn/img/common/wechat-pub.png" width="160" alt="Yideng AI WeChat official account QR code"></td>
  </tr>
</table>

Scan the official account QR code in WeChat for AI tools and productivity tips.

## Usage limits

- Intended for temporary transfers on trusted local networks; no remote transfer, automatic sync or backup.
- **Access token protection is off by default.** Each launch generates a new token and resets protection to off. Without protection, devices that can reach the service can browse, download and upload.
- Tokens restrict access but do not encrypt transfers. The service uses HTTP; avoid transferring sensitive files on public Wi-Fi.
- The shared folder and its non-hidden subfolders are accessible to visitors. Files and folders beginning with `.` are hidden. Use a dedicated folder; uploaded files also appear in the list.
- Opening a file or installing an app depends on the receiving device's supported formats and system restrictions.

## Troubleshooting

- **Phone cannot connect:** check the Wi-Fi network, VPN / proxy, firewall and router AP / client isolation settings.
- **Downloads fail in WeChat:** use the top-right menu to open the page in an external browser: Safari on iPhone or the system browser on Android.
- **Link exposed:** stop and restart the service, enable access token protection and share the new link. Restarting alone does not restrict access while protection is off.

## Run and build from source

Requires Go 1.22+:

```bash
git clone https://github.com/goldenwind/fangxu-file-transfer.git
cd fangxu-file-transfer
go run . --dir "/path/to/shared/folder"
```

| Flag | Description |
| --- | --- |
| `--dir path` | Shared folder; defaults to your user's `Downloads` |
| `--listen 0.0.0.0:9000` | Fixed port; defaults to an automatically selected free port |
| `--no-open` | Do not open the desktop browser automatically |

```bash
go test ./...
go build -o fangxu-file-transfer .
./build-packages.sh
```

The packaging script creates Windows, macOS and Linux packages in `dist`. Building all packages requires macOS with `lipo`, `sips`, `codesign` and `ditto`.

## License

Released under the [MIT License](./LICENSE). [Issues](https://github.com/goldenwind/fangxu-file-transfer/issues), suggestions and pull requests are welcome.

## Other file transfer methods and choosing a tool

Choose by device pair; `↔` means transfers in both directions.

| Platform ↔ platform | Suggested methods | HTTP / browser option |
| --- | --- | --- |
| Windows ↔ Android | [Quick Share](https://support.google.com/android/answer/13801258?hl=en), USB cable, LocalSend | **Fangxu File Transfer**: launch on the computer, scan to upload / download |
| Windows ↔ iPhone / iPad | LocalSend; SMB for persistent sharing | **Fangxu File Transfer**: launch on the computer, upload / download in Safari |
| macOS ↔ Android | LocalSend | **Fangxu File Transfer**: launch on the Mac, scan to upload / download |
| macOS ↔ iPhone / iPad | [AirDrop](https://support.apple.com/en-gb/119857), LocalSend | **Fangxu File Transfer**: launch on the Mac, upload / download in Safari |
| Linux ↔ Android | USB cable, LocalSend | **Fangxu File Transfer**: launch on the computer, scan to upload / download |
| Linux ↔ iPhone / iPad | LocalSend; SMB for persistent sharing | **Fangxu File Transfer**: launch on the computer, upload / download in Safari |
| Windows ↔ macOS / Linux | LocalSend; SMB / NAS for persistent sharing | **Fangxu File Transfer**: launch on one computer, upload / download in the other's browser |
| macOS ↔ Linux | LocalSend; SMB / NAS for persistent sharing | **Fangxu File Transfer**: launch on one computer, upload / download in the other's browser |
| Android ↔ Android | Quick Share, LocalSend | This project requires a computer as the server |
| iPhone / iPad ↔ iPhone / iPad | AirDrop, LocalSend | This project requires a computer as the server |
| Android ↔ iPhone / iPad | LocalSend | This project requires a computer as the server |

**For browser-based transfers on the same trusted LAN, use this project's HTTP option: [download Fangxu File Transfer](https://github.com/goldenwind/fangxu-file-transfer/releases/latest).** Share a folder from your computer, upload or download from other devices' browsers, and stop the service when finished. HTTP is unencrypted and access token protection is off by default; read “Usage limits” above before sharing.

[LocalSend](https://localsend.org/) requires clients on participating devices and supports encrypted cross-platform LAN transfers. Check the official Quick Share documentation for compatibility and client requirements. Files accessible over USB depend on the phone's system. For remote transfers, use cloud storage, email or messaging; storage limits, retention and compression depend on the service.

For Kindle devices or delivery to the Kindle cloud library, see [Fangxu Kindle Transfer](https://github.com/goldenwind/fangxu-kindle-transfer/blob/main/README_EN.md), covering browser downloads, Send to Kindle, mobile sharing and email delivery.
