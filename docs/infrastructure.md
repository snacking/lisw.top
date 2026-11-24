# 基础设施核查 · 2026-11-24
WSL2 Ubuntu 22.04；GitHub SSH 身份 snacking 验证成功；VPS root@198.12.74.38 验证成功。
源仓库：../sw_proxy_infra（git@github.com:snacking/sw_proxy_infra.git）。仅查看模板与容器元数据，没有复制线上凭据。

| 容器 | 镜像 | 部署情况 |
| --- | --- | --- |
| hy2 | tobyxdd/hysteria:v2 | host 网络、TLS 文件挂载、password 认证、HTTP masquerade |
| xray | ghcr.io/xtls/xray-core:26.6.27 | host 网络、VLESS、REALITY、Vision |
| nginx-fake | nginx:alpine | 8080 → 80；127.0.0.1:9443 → 9443 |
| uptime-kuma | louislam/uptime-kuma:1 | 3001 → 3001 |

核查时未见 TeamSpeak 容器；未更改任何在线服务。hy2 与 xray 的实际端口、密钥仍以未读取的线上配置为准。面板内端口和凭据全部为演示值。

Windows SSH 密钥已复制至 WSL ~/.ssh，目录权限 700，私钥与 config 600，公钥 644。首次连接使用 accept-new 写入 known_hosts（未通过独立渠道核对指纹）。不要将 ~/.ssh 或线上环境文件提交到仓库。
