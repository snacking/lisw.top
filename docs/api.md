# Mock API

请求和响应 JSON，认证使用 HttpOnly、SameSite=Strict 的演示会话 Cookie。登录身份选择是产品演示，不能视为真实鉴权。

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | /api/health | 模式与健康状态 |
| POST | /api/login | 任意邮箱密码，role=admin 进入管理员，否则 friend |
| POST | /api/register | 同登录，额外调用 Mock Mailer |
| POST | /api/logout | 结束会话 |
| GET | /api/snapshot | 当前用户可见入口、运行位置、服务、成员和动态 |
| GET | /api/events | SSE，refresh 事件后重新取快照，20 秒心跳 |
| POST | /api/services/{id}/start | 管理员模拟启动 |
| POST | /api/services/{id}/stop | 管理员模拟停止 |
| POST | /api/services/{id}/upgrade | 管理员模拟升级到 mock-latest |
| POST | /api/services/{id}/config | {revision:number, config:object}，管理员更新配置 |

错误体 {error:string}；401 未登录、403 无权限、404 不存在、409 配置版本冲突、422 配置字段错误。配置须有 host、credential、有效 port，协议字段由服务驱动扩展。实际 VPS 服务完全不接入 API。

内存重启回到种子状态；SSE 断开自动重连后重新获取快照。事件只发送无敏感内容的刷新信号，可见服务由快照接口重新判定。

## 持久化更新
配置已接入真实 MySQL；DATABASE_CONFIG_FILE 指向仓库外的私有连接文件。数据库 DDL/DML 与 TLS 配置集中在 backend/database。服务操作仍为 Mock，Redis 和邮件仍为 Mock。/api/health 和 /api/snapshot 增加 storage，值为 mysql 或 memory。数据库写失败返回 503，跨实例状态冲突返回 409。实例内即时 SSE，跨实例每 5 秒读取数据库并通知。
## 登录校验已启用（替代早期演示登录说明）
POST /api/login 请求 {username,password}（也兼容 email 字段），成功返回 userId、数据库角色和不含密码的 user。错误账号/密码统一返回 401；超频 429；数据库异常 503。无验证的注册已关闭，/api/register 返回 403。角色请求参数忽略，旧 demo_session Cookie 无效。会话 panel_session 的有效期为 8 小时，退出会清除；失效会话访问资源返回 401。账号停用后新请求也会拒绝。
