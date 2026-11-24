# 验证记录 · 2026-11-24

- WSL2 Ubuntu-22.04，原生 Linux Node v24.22.3 / npm 11.19.1 / Go 1.27.4；工具包按官方 SHA256 校验。
- GitHub SSH 返回 Hi snacking；VPS SSH 和 racknerd 别名均成功。
- 基础设施仓库克隆成功，工作区未改动；VPS 仅运行只读核查命令。
- go test -race ./...：通过，覆盖普通用户禁止写入、局部服务管理员隔离、无授权用户不可见、配置版本冲突、非法端口、注册不校验、SSE 配置更新通知。
- go vet ./... 与 go build：通过。
- 实际 HTTP 联调：登录、用户读取、用户写入 403、管理员修改、用户读取新版本，通过。
- GitHub Actions YAML 解析：通过；实际云端工作流未触发，缺少新项目 GitHub remote / Actions 配置。
- 浏览器视觉验收未完成：内嵌浏览器创建超时，Windows UI 自动化因无法可靠识别当前浏览器 URL 被停止。未绕过此限制。
- 前端 vue-tsc 与 Vite 生产构建通过，产物 JS 约 97KB（gzip 37KB）。
- 安装期间使用临时元数据适配器绕过超大历史元数据，esbuild 包按 registry SHA512 校验；最终锁文件统一使用官方 registry.npmjs.org，无临时代理依赖。
- 将锁文件改为官方地址后，npm ci --offline 与再次生产构建通过；Go 静态服务首页、JS、CSS 均返回 HTTP 200。

## MySQL、部署与本地访问更新
- WSL wslinfo --networking-mode=mirrored；Windows PowerShell 对 localhost:8081、127.0.0.1:8081、首页的请求均返回 200。原因为此前预览进程已停止，未修改防火墙或网络模式。
- 已启用 systemd 用户服务 lisw-panel-dev，预览不再依赖临时终端；scripts/dev.sh 构建后自动重启它。
- MySQL CA 与主机名校验的真实 TLS 连接成功；幂等 DDL 成功；API 写入后独立进程恢复相同配置与 revision，验证通过。
- 数据库不可用时恢复内存、跨实例同步触发 SSE 的竞态测试通过；go vet、生产构建、shell/YAML 检查通过。
- GitHub PAT 对 Actions secrets API 返回 403；未尝试将数据库密码写入 Git 或 artifact。
