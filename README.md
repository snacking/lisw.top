# lisw. 私人服务面板
Vue 3 + TypeScript + Vite / Go。仓库：git@github.com:snacking/lisw.top.git。
服务控制、升级和连接凭据仍为 Mock；MySQL 已真实接入并持久化状态。登录使用真实数据库账号与密码哈希校验，注册暂未开放。Redis 和邮件保持 Mock。

## WSL 本地预览
```bash
cd /home/sn4ck/workspace/Git/lisw.top
bash scripts/dev.sh
```
访问 http://localhost:8081 或 http://127.0.0.1:8081。已安装用户服务 lisw-panel-dev，终端退出仍可访问；`systemctl --user status lisw-panel-dev` 查看状态，`systemctl --user restart lisw-panel-dev` 重启，`systemctl --user stop lisw-panel-dev` 停止。DEV_FOREGROUND=1 bash scripts/dev.sh 可前台运行（先停止已有服务）。

WSL 实际为 mirrored，Windows 到 localhost 的 HTTP 已验证。此前无法访问源于预览服务已停止，不需要关闭防火墙或改网络模式。

## 数据库
数据库独立于 backend/database：mysql.go 管理连接、CA 校验 TLS、连接池和持久化；sql/ 存放 DDL 与参数化 DML，certs/ 仅保存公开 CA。
已建立用户、Endpoint、运行节点、服务及两级成员权限关系表，详见 docs/schema.md。已有 JSON 快照保留为兼容读模型；关系表和快照在同一事务中保存，并用全局 version 做乐观并发控制。部署时执行版本迁移、启动时检查结构并回填现有数据；多实例每 5 秒同步，实例内修改即时经 SSE 通知。

连接配置优先读取 DATABASE_CONFIG_FILE 指定的私有 JSON（字段 url、caFile）；也可用 DATABASE_URL、DATABASE_CA_FILE。本机私有文件位于 ~/.config/lisw-panel/database.json，权限 600，不在 Git 内。无数据库配置时仍支持内存演示，配置存在但连接失败则启动失败，不会静默退回内存。

## 构建与部署
```bash
bash scripts/build-release.sh   # 本机编译 Linux amd64 成品
bash scripts/deploy-local.sh    # 本机编译后 scp 到 racknerd
```
产物 artifacts/panel-linux-amd64.tar.gz 包含 Go 静态二进制、前端 dist、CA、systemd unit、校验和。VPS 仅校验、解包、重启；没有 Go/npm/Docker 编译。
部署位置 /opt/lisw-panel，服务 lisw-panel，监听 127.0.0.1:18081；CPUQuota 30%、MemoryMax 96M，独立于 sw_proxy_infra，完全不修改 nginx-fake 的 8080/9443 或 Docker Compose。
VPS 私有数据库配置 /etc/lisw-panel/database.json 由 systemd LoadCredential 传给动态用户，配置中的 caFile=/opt/lisw-panel/current/certs/aiven-ca.pem。
远程预览可用 `ssh -L 18081:127.0.0.1:18081 racknerd` 后访问 http://localhost:18081。

## GitHub Actions
main 推送和 PR 自动在 GitHub runner 上运行 Go 测试、前端构建、静态二进制编译并上传 artifact；手动 workflow_dispatch 勾选 deploy 时下载该 artifact 部署。mock-preview Environment（或仓库）需要 VPS_SSH_KEY、VPS_KNOWN_HOSTS 两个 secrets。VPS_HOST、VPS_USER 可选，默认 198.12.74.38、root。数据库密码已独立保存在 VPS，不需进入 Actions 构建或构建包。
GitHub Actions 与 Secrets 读写权限已验证；云端 CD 需要上述两个部署 secrets。

## 验证
```bash
cd backend && go test -race ./... && go vet ./...
cd ../frontend && npm run build
```
Endpoint 区分入口 IP/域名、Runtime 表示实际部署机器；端点管理员可管理全部服务，服务管理员只操作授权服务。服务接口后续可注册 Minecraft / TeamSpeak 驱动。成员管理写接口、真实认证和真实服务控制仍属后续范围。
详见 docs/interaction.md、docs/api.md、docs/infrastructure.md、docs/verification.md。

DDL 与 DML 已分别存放于 backend/database/sql/ddl、sql/dml。部署用 panel --migrate 显式执行带锁、版本和校验和的迁移；普通应用启动不执行 DDL。详见 docs/schema.md 的发布规则。

## 登录校验
用户名或邮箱 + 密码；账号必须 active、非演示账号且已设置 password_hash。角色只从数据库读取，不能通过请求参数选择。会话 Cookie 为 HttpOnly、SameSite=Strict，8 小时到期；退出立即失效，旧 demo_session 不再接受。每个来源 IP 每分钟最多 5 次登录请求。登录服务或数据库不可用时拒绝登录。未开放公开注册，/api/register 返回 403。密码不变，仍使用先前单独设置的账号密码。
