# EatBy

EatBy 是一个手机优先的家庭食材、库存、保质期和采购清单管理工具。每个家庭拥有独立数据空间，户主可以邀请家庭成员，并生成 Kindle/WebLaunch 黑白只读看板。

## 功能

- 平台邀请码创建独立家庭，用户名和密码登录
- 家庭内成员邀请、设备撤销和成员移除
- 平台管理后台：邀请码、家庭暂停、临时密码重置
- 冰箱、冷冻室、橱柜等自定义位置
- 数量模式和“充足/一半/快没了/用完”状态模式
- 同商品多批次、包装到期日、开封后期限和临期预警
- 条码建档、采购清单、低库存提醒和库存操作撤销
- 家庭隔离的 Kindle 只读链接
- SQLite WAL、每日一致性备份和 OSS 异地备份脚本

## 本地开发

需要 Go 1.24+、Node.js 24+ 和 Docker。

```bash
cp .env.example .env
# 修改 .env 中的平台管理员密码
cd frontend
npm install
npm run build
cd ..
EATBY_ADMIN_USERNAME=admin EATBY_ADMIN_PASSWORD='your-password' go run ./cmd/server -data ./dev-data
```

打开 `http://localhost:8080`，使用 `.env` 中的管理员账号登录，在管理后台生成首个家庭注册链接。

`make dev` 会加载仓库根目录的 `.env`。管理员密码只在首次创建账号时读取；修改 `.env` 后如需同步更新开发数据库中的管理员密码，执行：

```bash
make dev-reset-admin
make dev
```

常用入口：

- `/`：所有家庭成员登录。登录后由账号自动进入所属家庭，不使用可猜测的家庭 URL。
- `/admin`：平台管理员登录与家庭管理。
- `/register?code=...`：平台邀请码创建新家庭。
- `/join?token=...`：家庭邀请成员注册。
- `/display/<token>`：Kindle 家庭只读看板。

单家庭旧版 `data/pantry.db` 不会自动迁移。多家庭版本应使用新的空目录，例如 `dev-data` 或 `production-data`。

## 阿里云大陆部署

推荐购买阿里云轻量应用服务器：2 核、4GB 内存、60GB 以上 SSD、5Mbps 以上带宽、Ubuntu 24.04 LTS。实例防火墙只开放 22、80、443，22 端口限制为自己的公网 IP。

使用前确认域名 ICP 备案已经接入阿里云；若备案在其他云商，需要先完成接入备案。建议添加子域名：

```text
eatby.example.com  A  <服务器公网 IPv4>
```

服务器初始化：

```bash
sudo apt update
sudo apt install -y ca-certificates curl git
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
# 重新登录 SSH 后继续
sudo mkdir -p /opt/eatby
sudo chown "$USER:$USER" /opt/eatby
git clone https://github.com/GpigeonCX/EatBy.git /opt/eatby
cd /opt/eatby
cp .env.example .env
```

编辑 `/opt/eatby/.env`：

```dotenv
DOMAIN=eatby.example.com
EATBY_DATA_DIR=/opt/eatby/production-data
EATBY_IMAGE=eatby:local
EATBY_ADMIN_USERNAME=admin
EATBY_ADMIN_PASSWORD=使用密码管理器生成的长随机密码
EATBY_ICP_NUMBER=京ICP备xxxxxxxx号
OSS_BUCKET=your-private-backup-bucket
```

首次发布：

```bash
cd /opt/eatby
docker compose up -d --build
docker compose ps
curl -fsS "https://$(grep '^DOMAIN=' .env | cut -d= -f2)/healthz"
```

Caddy 会自动申请 HTTPS 证书。应用端口 8080 不对公网暴露，SQLite 数据保存在 `/opt/eatby/production-data`。

## 手动更新与回滚

发布前先等待或手工保留一份 `production-data/backups/` 中的数据库快照，然后执行：

```bash
cd /opt/eatby
git pull --ff-only
docker compose build --pull
docker compose up -d
docker compose ps
```

建议每次稳定发布创建 Git 标签。回滚代码时切换到旧标签并重新构建；如新版本修改过数据库结构，同时恢复发布前 SQLite 备份。

## OSS 异地备份

应用每天凌晨 3 点生成一致性 SQLite 快照，本机保留 14 份。安装并配置阿里云 `ossutil` 后，可在服务器加入凌晨 4 点的定时任务：

```cron
0 4 * * * cd /opt/eatby && set -a && . ./.env && set +a && ./deploy/backup-to-oss.sh >> /var/log/eatby-backup.log 2>&1
```

OSS Bucket 应设为私有，并配置 30 天生命周期规则。至少执行一次从 OSS 下载备份并启动临时实例的恢复演练。

## GitHub 推送

仓库远端已经配置为 `git@github.com:GpigeonCX/EatBy.git`。如果网络屏蔽 GitHub SSH 22 端口，在 `~/.ssh/config` 中添加：

```sshconfig
Host github.com
  HostName ssh.github.com
  User git
  Port 443
```

然后执行：

```bash
ssh -T git@github.com
git push -u origin main
```

## 自动发布

CI 会在每次 PR 和 `main` 推送时运行前后端测试及 Docker 构建。Release 工作流会把镜像发布到：

```text
ghcr.io/gpigeoncx/eatby:<commit-sha>
ghcr.io/gpigeoncx/eatby:latest
```

稳定后，在 GitHub 仓库配置：

- Actions Variable：`AUTO_DEPLOY=true`
- Actions Secrets：`DEPLOY_HOST`、`DEPLOY_USER`、`DEPLOY_SSH_KEY`

之后 `main` 构建成功会通过 SSH 更新 `/opt/eatby` 并部署对应提交镜像。生产 `.env`、数据库和 OSS 凭据始终只保存在服务器。

## 验证

```bash
make test
docker build -t eatby:test .
docker compose config
```

平台管理员默认只能查看家庭名称、账号和用量状态，管理接口不提供家庭具体食材内容。隐私说明位于 `/privacy`。
