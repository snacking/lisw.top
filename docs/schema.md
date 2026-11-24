# 数据模型

以稳定 ID 建关联，IP/域名只是 Endpoint 的可变地址，不能作为用户权限的外键。当前单工作空间模型使用 lisw_ 前缀，避免影响同数据库的其他应用。

| 表 | 关键字段 | 约束 / 用途 |
| --- | --- | --- |
| lisw_users | id, username, display_name, github_username, email, password_hash, platform_role, status, is_demo | 用户名、GitHub 用户名、非空邮箱唯一；未设置的邮箱和密码为 NULL |
| lisw_endpoints | id, name, address, address_type, region, provider, owner_user_id | IPv4 / IPv6 / 域名；地址唯一，owner 外键限制删除 |
| lisw_runtimes | id, name, location, host_address | 实际运行机器，与入口独立，可为其他电脑或远程节点 |
| lisw_services | id, endpoint_id, runtime_id, kind, name, status, version, revision, config, control_mode | 入口与运行位置外键；kind 可扩展；config JSON；控制模式 mock/managed |
| lisw_endpoint_members | endpoint_id, user_id, role | 联合主键；入口管理员/用户 |
| lisw_service_members | service_id, user_id, role | 联合主键；服务管理员/用户 |
| lisw_panel_state | id, version, document | 兼容现有 API 的快照读模型和事件历史 |
| lisw_relational_sync | id, source_version | 关系表对应的快照版本，兼容升级前的进程写入 |

所有业务实体带创建/更新时间；服务保留既有 updatedAt 和 revision；成员表按用户索引以便查询授权资源。用户删除受外键保护，资源成员关系跟随资源删除。

```mermaid
erDiagram
  lisw_users ||--o{ lisw_endpoints : owns
  lisw_users ||--o{ lisw_endpoint_members : joins
  lisw_endpoints ||--o{ lisw_endpoint_members : grants
  lisw_endpoints ||--o{ lisw_services : exposes
  lisw_runtimes ||--o{ lisw_services : runs
  lisw_users ||--o{ lisw_service_members : joins
  lisw_services ||--o{ lisw_service_members : grants
```

## 初始化记录
- 用户 id=admin，username=snacking，display_name=Snacking，github_username=snacking，platform_role=admin，is_demo=false。沿用 admin ID 保持已有会话与授权兼容；邮箱和密码未设置，不猜测个人信息。登录注册仍按既定需求暂不验证。
- Endpoint id=racknerd，地址 198.12.74.38，类型 ipv4，provider=RackNerd，region=美国 · 洛杉矶，owner_user_id=admin。
- Runtime id=vps，host_address=198.12.74.38，表示该 VPS 本机部署。
- hy2 / Hysteria 2 与 vless / VLESS · Reality 从现有快照迁移，保留当前配置、版本、revision、成员权限；仍为 mock 控制，未读取或写入真实服务密钥。
- 既有 friend 演示成员保留，is_demo=true；不将其视为新注册的真实用户。

## 写入和迁移
DDL 独立存放于 backend/database/sql/002–008，参数化 DML 单独存放在同目录。启动时幂等创建表并初始化所有者，首次 Load 在事务中锁住 workspace 快照行后回填关系表。之后 Save 的版本 CAS、关系表更新和同步版本标记在同一个事务提交；失败全部回滚。旧版本实例只更新快照时，新实例按 source_version 检测并补齐，避免滚动部署时漏数据。

API 暂时继续通过快照读模型工作，因此直接手动修改关系表不会自动反馈到 API；应通过应用写入以保持两者一致。用户资料不随快照覆盖或删除。当前没有实现用户管理/成员管理写 API，后续可在同一事务中扩展，不应维护两套独立数据源。

检查：go test ./database；显式执行线上集成检查：LISW_DB_INTEGRATION=1 DATABASE_CONFIG_FILE=~/.config/lisw-panel/database.json go test ./database -run TestLiveRelationalMigration -v。
