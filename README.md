# 家庭食材

一个手机优先的家庭食材、库存、保质期和采购清单管理工具。前端是可安装的 PWA，后端使用 Go 和 SQLite，并提供适合越狱 Kindle/WebLaunch 的黑白只读看板。

## 已实现

- 冰箱、冷冻室、橱柜等自定义位置
- 数量模式和“充足/一半/快没了/用完”状态模式
- 同商品多批次、包装到期日、开封后期限和临期预警
- 商品条码首次建档、再次扫码快速入库
- 采购清单、低库存一键加入、最近库存操作撤销
- 户主密码、一次性成员邀请、逐设备会话撤销
- 可撤销的 Kindle 只读链接及五分钟自动刷新
- PWA 安装与最近看板缓存、SQLite 每日备份

## 本地运行

需要 Go 1.24+ 和 Node.js 24+。

```bash
cd frontend
npm install
npm run build
cd ..
go run ./cmd/server -data ./data
```

打开 `http://localhost:8080`，首次访问会进入家庭初始化页面。开发前端时可分别运行后端和 `cd frontend && npm run dev`。

## 公网部署

准备一台安装了 Docker 的 VPS，将域名解析到服务器，然后执行：

```bash
export DOMAIN=pantry.example.com
docker compose up -d --build
```

Caddy 会自动申请 HTTPS 证书。数据位于 Docker 的 `pantry-data` 卷，应用每天凌晨 3 点在卷内的 `backups/` 目录创建 SQLite 备份，保留 14 份。

## Kindle

户主在“设置 → Kindle 看板”中生成链接，在 Kindle WebLaunch 或浏览器中全屏打开。看板令牌只能读取摘要，不能修改数据；设备丢失后可在设置页撤销令牌。

## 验证

```bash
make test
```
