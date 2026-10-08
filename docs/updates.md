# 客户端检查更新

“关于与帮助”中的“检查更新”手动调用 `gf_api` 的公开产品升级接口，无需登录：

```http
GET https://api.ip21.cn/api/products/10/releases/latest?current_version=1.1.1&channel=stable&platform=macos&arch=aarch64&installation_id=随机UUID
```

产品 ID `10` 与产品反馈保持一致。协议依据 `gf_api/docs/product_upgrade_api.md`、`app/api/product_release.go` 和 `app/model/product_release.go`。

- 当前版本来自原生客户端包信息，平台和架构来自编译目标。服务端会将 macOS 的 `macos`、Windows 的 `windows`、Linux 的 `linux` 以及常见架构别名归一化。
- 随机安装 UUID 首次检查时创建，保存在系统应用配置目录的 `installation-id` 文件中，供灰度发布稳定分桶使用。不发送文件、目录或硬件标识。
- 原生 Rust 请求连接超时为 5 秒，总超时为 15 秒，限制响应为 256 KiB。服务异常、网络失败和无效响应均显示失败，可再次检查。
- 无更新时显示“当前已是最新版本”；有更新时显示版本号、发布标题、纯文本更新说明与下载按钮。`force_update` 会显示需要升级的提示。
- 下载按钮通过系统浏览器打开服务端的 HTTPS `download_url`；无有效下载地址时使用 HTTPS `page_url`。安装由用户手动完成。
- 检查和下载入口独立于局域网传输服务状态，中英文切换会同步更新提示。网页预览中显示桌面客户端提示并禁用检查按钮。

发布新版本时，在 `gf_api` 后台的“产品版本升级”中为产品 `10` 配置发布记录，设置 `stable` 渠道、对应平台/架构、比当前版本高的语义版本、HTTPS 安装包地址及更新说明。记录需启用并到达发布时间才会返回给客户端；灰度比例通过安装 ID 判定。未配置匹配的新发布时，接口仍返回成功，但 `update_available` 为 `false`。
