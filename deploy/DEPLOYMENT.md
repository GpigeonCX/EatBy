# 发布与回滚

## 当前生产状态（2026-10-20 核对）

- 域名：`https://met.xin`；源码：`/opt/eatby`。
- Docker Compose：`eatby-app-1` + `eatby-caddy-1`；数据：`/opt/eatby/production-data`。
- 当前运行 `eatby:local`。Kindle 开发阶段是本地交叉编译 linux/amd64 Go 程序，
  上传到服务器，再叠加到旧运行镜像并加入 Noto CJK 字体；只重启 app。
- 那个临时发布流程不在 GitHub Actions 中。材料在 `/opt/eatby-kindle-release/`，
  发布前源码/SQLite 备份在 `/opt/eatby-backups/`。不要把它当作后续常规发布入口。
- GitHub `AUTO_DEPLOY=true`，三个 SSH secrets 已配置。**服务器源码仍有未提交修改，
  需先完成下面一次性整理，否则自动发布的 git pull 可能冲突。**
- 本次整理提交使用 `[skip deploy]`：仍跑测试、发布镜像，但不进入 SSH 部署 job。
  未修改生产容器、数据库或服务器工作区。

## 一次性整理云端源码

SSH 登录服务器（或使用 Workbench），在发布无人并发进行时操作：

```bash
cd /opt/eatby
git status --short
# 保存开发时同步的已跟踪和未跟踪源码；不会包含 .gitignore 忽略的 .env/数据目录。
git stash push --include-untracked -m "before-repository-sync-$(date +%Y%m%d-%H%M%S)"
git pull --ff-only origin main
git status --short
```

保留这个 stash，必要时用 `git stash show --stat` 查看；不要直接 pop 回已更新源码。
不要 `git reset --hard`、`git clean -fdx` 或删除 production-data。这一步仅同步源码，
不会更新正在运行的容器。若 pull 失败，先解决 GitHub 连接/鉴权或分支分歧，勿强制覆盖。

## 推荐日常流程：推送 main 自动发布

本地修改业务代码后：

```bash
(cd frontend && npm ci && npm test && npm run build)
go test ./...
python3 -m unittest discover -s deploy/kindle -p 'test_*.py' -v
git add <本次修改的文件>
git commit -m "feat: describe the change"
git push origin main
```

Release 工作流（`.github/workflows/release.yml`）：

1. 前端测试/构建、Kindle 脚本模拟测试、带 Noto CJK 字体的 Go 测试。
2. Dockerfile 从源码重建前后端；推送 `ghcr.io/gpigeoncx/eatby:<完整提交 SHA>` 和 `latest`。
3. 若 `AUTO_DEPLOY=true` 且提交消息不含 `[skip deploy]`，SSH 到 `/opt/eatby`，
   `git pull --ff-only`，拉取 SHA 镜像，Compose 更新服务，最后检查公网 `/healthz`。
4. 在 GitHub Actions 查看 **Release 的 deploy job**，不要只看到 CI 绿色就认为已发布。
   也可使用 `gh run list --workflow release.yml` / `gh run view <run-id> --log-failed`。

只提交代码、不动线上：在提交消息中加入 `[skip deploy]`。这不是 `[skip ci]`。
`workflow_dispatch` 手动执行 Release 仍会按 AUTO_DEPLOY 配置部署，跳过标记只针对 push。

### 当前自动发布的边界

- 没有自动数据库快照、自动健康失败回滚、发布串行锁；重要数据迁移前须手工备份，
  避免同时推送多个生产发布。健康检查失败不代表旧服务已恢复。
- SSH 使用 ssh-keyscan 获取主机密钥，尚未配置固定指纹校验。
- GHCR 拉取需服务器网络畅通；私有包还需服务器预先登录 GHCR。
- SHA 镜像通过命令级 `EATBY_IMAGE` 传入，**不会写回生产 `.env`**；当前 `.env`
  仍是 `eatby:local`。自动发布后直接运行不指定镜像的 `docker compose up -d`，
  可能切回旧本地镜像。维护时先用下面命令查当前镜像，并显式指定它：

```bash
docker inspect eatby-app-1 --format '{{.Config.Image}}'
# IMAGE 填上面输出的镜像，不要凭记忆使用 latest。
EATBY_IMAGE="$IMAGE" docker compose up -d --no-deps --no-build app
```

## 手动发布：服务器源码构建（GHCR 不通时可用）

先完成源码整理；确保没有自动发布并发运行。使用这种方式前建议在 GitHub 将
`AUTO_DEPLOY` 暂时设为 `false`，发布完成后再决定是否恢复。

### 发布前备份

```bash
cd /opt/eatby
stamp=$(date +%Y%m%d-%H%M%S)
backup=/opt/eatby-backups/manual-$stamp
mkdir -p "$backup" && chmod 700 "$backup"
docker inspect eatby-app-1 --format '{{.Image}}' > "$backup/image-id.txt"
docker tag "$(head -n 1 "$backup/image-id.txt")" "eatby:rollback-$stamp"
printf '%s\n' "eatby:rollback-$stamp" > "$backup/rollback-image.txt"
# SQLite WAL 模式必须用一致性备份，不能只 cp 正在使用的 pantry.db。
python3 - "$backup/pantry.db" <<'PY'
import sqlite3, sys
src = sqlite3.connect('/opt/eatby/production-data/pantry.db')
dst = sqlite3.connect(sys.argv[1])
src.backup(dst)
assert dst.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
dst.close()
src.close()
PY
```

以上任一步失败就停止发布。备份目录含家庭数据，不要上传仓库。

### 更新与检查

```bash
cd /opt/eatby
git pull --ff-only origin main
EATBY_IMAGE=eatby:local docker compose build app
# 构建成功才执行下一条；只重启 app，不动 Caddy。
EATBY_IMAGE=eatby:local docker compose up -d --no-deps --no-build app
docker compose ps
# 启动/迁移可能需要时间；等 app healthy 再确认外部路径。
curl -fsS https://met.xin/healthz
docker compose logs --tail=100 app
```

Dockerfile 自动构建前端并装好中文字体；不需要在云端手工 npm build。
源码构建仍依赖 Docker Hub、Alpine/npm/Go 网络，不是完全离线发布。

### 回滚

```bash
cd /opt/eatby
# 替换为本次备份中 rollback-image.txt 的真实镜像名。
EATBY_IMAGE=eatby:rollback-YYYYMMDD-HHMMSS docker compose up -d --no-deps --no-build app
curl -fsS https://met.xin/healthz
```

回滚镜像不等于回滚数据库。新增兼容表通常不必恢复数据库；破坏性迁移则需停 app、
确认兼容性和会丢失的发布后写入，再从一致性备份恢复，正确处理 WAL/SHM 后启动。
不要在运行中的数据库上直接覆盖文件。长期使用回滚镜像时应同步维护 `.env` 的镜像配置。

## 哪些更新需要动 Kindle？

- 业务 API、手机页面、PNG 内容/布局：发布云端即可；Kindle 下次连接时获得新图。
- 手机修改时间/方向：无需发布；设备下次连接读取新配置，休眠时不能立即接收。
- `deploy/kindle/*.sh`：GitHub/云端发布**不会**更新设备。需要通过 USB/KUAL 安装；
  操作前先安全停止看板，勿覆盖正在执行的脚本。参见 [Kindle 操作说明](kindle/README.md)。
