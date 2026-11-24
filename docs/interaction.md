# 交互与动画指南

依据 Apple HIG 的 Motion、Accessibility、Design principles，转译为 Web 面板的项目规则。
https://developer.apple.com/design/human-interface-guidelines/motion
https://developer.apple.com/design/human-interface-guidelines/accessibility

## 信息架构
Endpoint 是朋友连接的稳定入口，可包含 IP 或域名；Runtime 是实际服务运行机器；Service 关联入口与运行位置。Endpoint 管理员管理成员和所有服务，Endpoint 用户仅有入口可见性；Service 成员单独授权，管理员可修改配置和控制状态，用户可查看连接信息。权限以 API 判定为准。

## 视觉
暖灰背景、白色内容卡、石墨色侧栏、克制的青绿色强调；状态始终有文字，不只靠颜色。8px 间距体系，圆角 16–24px，正文 14–16px，重要数字 28–40px。桌面双栏，窄屏单栏。按钮至少 44px；键盘焦点清晰，表单均有标签。

## 动画
悬停与按压 140ms；面板切换与反馈 220ms ease-out；透明度和小幅位移表达层级，避免持续闪动、视差和装饰性循环。prefers-reduced-motion 下取消位移和过渡。刷新不抢焦点；异步操作保留原内容，显示忙碌状态并禁止重复提交。

## 状态与反馈
连接配置默认遮蔽，可主动显示或复制；复制成功仅在 Clipboard API 成功后提示。停止服务需确认；更新配置先展示可编辑草稿再保存。失败保留输入并提供错误信息。实时更新通过 SSE，断线显示重连状态，重连后读取快照；版本和更新时间可见。登录注册按需求不验证，显著标识演示模式。演示身份不是真实认证，禁止将本版公开用于真实凭据。
