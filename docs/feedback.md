# 产品反馈

客户端左侧底部的「产品反馈 / Product Feedback」打开系统默认浏览器，进入：

https://api.ip21.cn/products/10/feedback

传输功能免费，无需账号。提交反馈时，网页使用已有的「一灯 AI」公众号登录：获取 5 位验证码，向公众号发送数字，网页自动确认登录。可以选择问题分类，填写标题、复现描述，附上截图（PNG/JPEG/GIF，最大 5 MB）和可选联系邮箱。该产品不显示支付与会员分类。

反馈内容与主动选择的截图会提交到产品反馈服务器；共享目录和传输文件不会随反馈入口上传。反馈页面只有用户提交后才保存问题。登录验证使用原有鉴权机制。

## ECS 配置

- 产品：`方序传文件`，ID：`10`，状态：启用。
- 产品标识：`fangxu_file_transfer`。
- 登记脚本：[register-product.sql](./deploy/register-product.sql)，幂等执行，仅写入产品表，不创建会员或售价记录。
- 服务沿用 `gf_api` 的产品反馈模块，页面模板为 `resource/template/feedback/product.tpl`。
- 反馈网页新增公众号验证码登录、过期刷新与轮询；已携带登录令牌的产品继续直接使用表单。
- `api.ip21.cn` 的 Nginx 扩展配置 `fangxu-feedback-login.conf` 将两个精确 GET 路径 `/api/users/verify-code` 和 `/api/users/login-status` 转发至原有登录接口，保持同源请求。配置内容见 [feedback-login.nginx.conf](./deploy/feedback-login.nginx.conf)。登录状态查询关闭访问日志，响应禁止缓存。
- 原模板已保留部署前备份；无需覆盖其他后台代码或重启 Go 服务。

管理员可在 `https://api.ip21.cn/admin/info/product-feedback` 按产品筛选和处理反馈。
