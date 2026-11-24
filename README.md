# lisw. 私人服务面板
Vue 3 + TypeScript + Vite / Go 标准库。当前版本是完整交互的内存 Mock，重启清空修改。不验证登录注册，演示页可选择管理员；不可用于真实认证或真实凭据。

## 本地运行（WSL）
```bash
cd /home/sn4ck/workspace/Git/lisw.top
source ~/.local/toolchains/env
cd frontend && npm ci && npm run build
cd ../backend && go run .
# http://localhost:8081
```
开发时两个终端分别运行 `cd backend && go run .` 和 `cd frontend && npm run dev`，访问 http://localhost:5173。Vite 代理 /api 到 Go。

## 功能
总览、入口、服务配置、显示/复制连接 URI、管理员配置编辑/启停/模拟升级、成员权限、活动记录、SSE 实时同步与自动重连、响应式布局、减少动态效果适配。凭据是虚构数据，复制链接不能连接真实 VPS。

Endpoint 表示 IP/域名入口，Runtime 表示部署位置，Service 连接两者。Endpoint 管理员拥有全部服务管理权限；Endpoint 用户需额外有服务成员权限。ServiceDriver 注册表现支持 Hysteria2/VLESS mock；可增加 Minecraft、TeamSpeak 和远程代理驱动。Repository/Cache/Mailer 是后续真实依赖的替换边界；本版状态存储 Memory，数据库持久化未实现。SSE 只发刷新信号，读取快照时检查权限。配置保存携带 revision，冲突返回 409。

## 检查
```bash
cd backend && go test -race ./... && go vet ./...
cd ../frontend && npm run build
```

## GitHub Actions
推送 main 或 PR 执行前后端构建、Go 权限/冲突测试并打包 Linux artifact。手动 workflow_dispatch 勾选 deploy 才部署独立 mock 预览：GitHub Environment `mock-preview` 配置 secrets `VPS_SSH_KEY`、`VPS_KNOWN_HOSTS`，variables `VPS_HOST=198.12.74.38`、`VPS_USER`（具有免密 sudo 的部署账号）。known_hosts 从已可信的 SSH 指纹建立，不能在 CI 中临时无条件信任。部署至 /opt/lisw-panel，不修改现有 Docker 服务；失败恢复旧 symlink。

默认部署只监听 VPS 127.0.0.1:18081，通过 `ssh -i ~/.ssh/id_racknerd -L 18081:127.0.0.1:18081 root@198.12.74.38` 预览。本阶段不配置公开反代，因为用户明确要求不校验登录。仓库尚无远程地址，工作流文件准备好后需推送到你的 GitHub 仓库并配置以上环境变量，才能实际执行。

详见 docs/interaction.md 与 docs/infrastructure.md。生产化需实现真正身份认证、成员管理写接口、持久化、密钥保护、审计、真实驱动与任务执行队列；本次不包含这些真实集成。
