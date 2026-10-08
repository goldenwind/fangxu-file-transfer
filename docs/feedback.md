# 产品反馈

客户端左侧底部的「产品反馈 / Product Feedback」在应用内的独立 WebView 窗口中打开：

https://api.ip21.cn/products/10/feedback

反馈窗口重复打开时会复用现有页面，保留正在填写的内容和登录状态；窗口标题跟随客户端的中英文选择。关闭反馈窗口不会停止传输服务。反馈窗口不授予本机服务控制权限，截图选择使用网页原生文件输入。浏览器预览中的入口仍打开网页。

传输功能免费，无需账号。提交反馈时，网页使用已有的「一灯 AI」公众号登录：获取 5 位验证码，向公众号发送数字，网页自动确认登录。可以选择问题分类，填写标题、复现描述，附上截图（PNG/JPEG/GIF，最大 5 MB）和可选联系邮箱。

反馈类型统一为「功能故障」「功能建议」「登录与购买」「其他反馈」，由反馈服务提供。

桌面客户端打开反馈页时自动携带 `app_version`（当前应用版本）、`os`（`macos` / `windows` / `linux`）、`os_version`（系统版本）和 `arch`（CPU 架构）。应用版本读取 Tauri 的运行时包信息，系统信息在 Rust 原生端读取，不通过浏览器信息推断。Linux 系统版本包含发行版名称，Windows 包含系统名称及版本；架构优先读取系统信息，无法读取时使用客户端构建架构。无法读取的系统版本不传入，不影响打开反馈页。

参数使用表单 URL 编码放在 URL 片段中，例如：

```text
https://api.ip21.cn/products/10/feedback#app_version=1.1.1&os=macos&os_version=15.6.1&arch=aarch64
```

反馈页读取后立即清除片段，展示环境信息，并在用户提交时一起保存；登录及提交成功后的表单重置都会保留这些信息。字段按服务端字符数上限裁剪（64 / 32 / 128 / 32），避免超长环境值导致反馈提交失败。直接访问不带参数的网页仍可正常反馈。客户端无需登录，账号鉴权继续由反馈网页完成，不使用局域网文件访问令牌。

反馈内容、软件环境与主动选择的截图会提交到产品反馈服务器；共享目录和传输文件不会随反馈入口上传。反馈页面只有用户提交后才保存问题。登录验证使用原有鉴权机制。

## ECS 配置

- 产品：`方序传文件`，ID：`10`，状态：启用。
- 产品标识：`fangxu_file_transfer`。
- 登记脚本：[register-product.sql](./deploy/register-product.sql)，幂等执行，仅写入产品表，不创建会员或售价记录。
- 服务沿用 `gf_api` 的产品反馈模块，页面模板为 `resource/template/feedback/product.tpl`。
- 反馈网页新增公众号验证码登录、过期刷新与轮询；已携带登录令牌的产品继续直接使用表单。
- `api.ip21.cn` 的 Nginx 扩展配置 `fangxu-feedback-login.conf` 将两个精确 GET 路径 `/api/users/verify-code` 和 `/api/users/login-status` 转发至原有登录接口，保持同源请求。配置内容见 [feedback-login.nginx.conf](./deploy/feedback-login.nginx.conf)。登录状态查询关闭访问日志，响应禁止缓存。
- 软件环境支持遵循 `gf_api/docs/product_feedback.md`：服务端部署新版前，已有反馈表须执行 `manifest/migrations/2026-10-08_product_feedback_environment.sql`，补齐软件环境列，并部署支持环境字段的服务代码和反馈模板。

管理员可在 `https://api.ip21.cn/admin/info/product-feedback` 按产品筛选和处理反馈。

## 验证

`cargo test --locked --manifest-path src-tauri/Cargo.toml` 验证反馈 URL 的目标、环境字段、特殊字符编码、长度限制和原生环境读取。反馈页参数读取、登录前保留、提交和重置行为可在 `gf_api` 中用 `node --test resource/template/feedback/product_test.cjs` 验证。
