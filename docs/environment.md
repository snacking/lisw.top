# WSL 开发环境

Ubuntu-22.04 / WSL2 x86_64，账号 sn4ck。
工具链安装在 ~/.local/toolchains，采用官方元数据、SHA256 校验后的 Linux 二进制，无需修改系统 Node 或 Go。
~/.profile 和 ~/.bashrc 加载 ~/.local/toolchains/env；当前终端执行 source ~/.local/toolchains/env。
SSH 使用 ~/.ssh/config 中 github.com 和 racknerd，分别指定独立密钥与 IdentitiesOnly。
GitHub 测试：ssh -T git@github.com（成功欢迎消息时退出码 1 是正常行为）。
VPS 测试：ssh racknerd hostname。
