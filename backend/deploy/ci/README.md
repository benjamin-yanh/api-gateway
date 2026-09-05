# GitHub Actions 生产部署

入口：`.github/workflows/deploy-production.yml`（Actions → Deploy production）。
工作流分支选择 `main`，`source_sha` 填写要发布的完整提交 SHA，再点击 Run workflow；`full` 更新控制端、中转端和前端，
`frontend` 只更新静态前端。默认 SHA 是当前线上发布提交 `8132920459b707551224649fc1dc60780b5855a0`，避免默认分支旧代码覆盖线上功能。构建产物按该 SHA 命名，部署脚本来自工作流分支。
不会在 push 或 PR 时自动发布，也不会更改返现开关或价格配置。

## 一次性接入

1. 将工作流提交至 GitHub。`workflow_dispatch` 的工作流必须存在于默认分支，
   才会在 Actions 中出现手动运行入口；之后在入口填写 `source_sha`。
2. 在仓库 Settings → Environments 创建 `production` 环境。
3. 在该环境配置 Variables：

   | 名称 | 当前值 |
   | --- | --- |
   | `CONTROL_HOST` | `<control-host>` |
   | `RELAY_HOST` | `<relay-host>` |
   | `PUBLIC_URL` | `https://<public-domain>` |

4. 在同一环境配置 Secrets：

   | 名称 | 内容 |
   | --- | --- |
   | `SSH_PASSWORD` | 本机环境同名变量，现有两台主机 root 登录密码 |
   | `DEPLOY_SSH_KEY`（可选） | 用于两台主机 root 登录的专用 SSH 私钥全文 |
   | `DEPLOY_KNOWN_HOSTS` | 已核对的两台主机 SSH known_hosts 记录 |

   当前使用密码认证。若配置私钥，则优先使用私钥，其公钥需已在两个主机的 root `authorized_keys` 中。密码仅通过 `SSHPASS` 环境变量传给 sshpass，不放在命令行参数中。
   不要把密码、私钥写进仓库。工作流使用严格主机指纹验证，不临时信任扫描结果。
   可在本机用 `ssh-keygen -F <control-host>` 和
   `ssh-keygen -F <relay-host>` 查看既有可信记录。
5. 主机需具备 bash、flock、sha256sum、tar、curl、systemctl；控制主机还需 nginx。
   主机 SSH 入口需允许所选 GitHub runner 访问。

当前工作流复用仓库已固定版本的 Actions、Bun 和 Go 模块版本。
可按团队流程在 production 环境配置允许发布的分支和审核者。

## 完整部署流程

1. 前端类型检查、lint、测试和构建；完整发布还执行后端测试、独立 relaykit
   构建，再构建锁定角色的 Linux amd64 二进制。
2. 上传构建产物到 GitHub，保留 14 天；构建任务不读取生产 Secrets。
3. 所有文件上传至对应主机 `/opt/new-api/releases/<run>-<attempt>-<sha>/`，
   每个主机只收到所需文件，激活前校验 SHA-256。
4. 更新控制端，轮询 `127.0.0.1:3001/healthz`，最长 15 分钟。
   启动会检查生产数据库，不要在等待期间手动重复重启。
5. 更新中转端，轮询 `127.0.0.1:3002/healthz`。
6. 两端就绪后更新前端，设置目录 755、文件 644；请求公网 `/pricing`，
   比较返回页面与新构建的 index.html。
7. 检查公网 `/healthz` 和 `/api/status`，在运行摘要中记录发布 ID。

工作流使用固定 production concurrency group，不取消进行中的发布；
服务器还用 flock 防止两个激活脚本同时操作相同安装目录。
手工部署也应使用这些脚本，避免绕过互斥。

## 回滚边界

每个组件的旧版本保存在该发布目录的 `backups/control`、`backups/relay`
或 `backups/frontend`。不自动清理历史备份。

组件激活失败时，脚本恢复该组件的旧版本并使 CI 失败。二进制恢复后会
尝试重启旧服务，仍需检查其就绪情况。已经成功发布的其他组件不会自动撤回，
数据库表、返现账本和余额从不回退或删除。需要回滚整个版本时，应先按
`AGENT_HANDOFF.md` 核对数据和在途请求兼容性。

特别是已有无上限返现请求时，不能直接回退到不支持该规则快照的版本。
手动取消任务、主机断电或 SSH 中断可能使状态不确定；先查看日志、服务状态和
备份，再决定恢复或重新发布。重新运行工作流会使用新的 attempt 目录，
不会覆盖原备份。

## 本地验证

```bash
bash -n backend/deploy/ci/activate.sh backend/deploy/ci/publish.sh
python3 backend/deploy/ci/test_activate.py
```

测试使用临时安装目录和替代服务命令，验证成功发布、校验失败、服务失败及
前端失败回滚，不连接生产服务器。首次真实 CI 运行仍需验证 runner 的网络、
Secrets 和生产环境配置。
