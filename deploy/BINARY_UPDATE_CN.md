# Sub2API 二进制部署更新文档

本文档适用于已经使用以下方式部署的服务器：

- `Sub2API` 以二进制形式运行
- 由 `systemd` 管理服务
- 使用 `Caddy` 反向代理
- `PostgreSQL` 与 `Redis` 运行在宿主机

本文档基于一台实际已部署服务器整理，当前结构如下：

```text
公网域名
  ↓
Caddy (80/443)
  ↓ reverse_proxy
127.0.0.1:2127
  ↓
/opt/sub2api/sub2api
  ↓
localhost:5432 PostgreSQL
localhost:6379 Redis
```

## 当前部署特征

可用以下命令确认当前服务器是否属于本文档适用范围：

```bash
systemctl status sub2api --no-pager
systemctl cat sub2api
ss -lntp | grep sub2api
cat /opt/sub2api/config.yaml
cat /etc/caddy/Caddyfile
```

典型特征：

- 二进制程序路径：`/opt/sub2api/sub2api`
- 配置文件路径：`/opt/sub2api/config.yaml`
- 服务名：`sub2api`
- 监听地址：`127.0.0.1:2127`
- 反向代理：`Caddy`

## 一次完整更新流程

以下流程按“从服务器获取最新代码，到启动新版应用”为顺序编排。

### 1. 准备服务器构建环境

如果服务器尚未安装构建工具，先安装：

```bash
apt update && apt install -y golang-go nodejs npm
npm install -g pnpm
```

确认版本：

```bash
go version
node -v
npm -v
pnpm -v
```

### 2. 确认服务器可通过 SSH 访问 GitHub

```bash
ssh -T git@github.com
```

如果返回类似：

```bash
Hi xuseny/codexApi! You've successfully authenticated...
```

说明可正常使用 deploy key 拉取代码。

### 可选：先从本地提交并推送代码到 GitHub

如果服务器上的代码需要先同步你本地的最新修改，请先在本地项目目录执行提交和推送，再回到服务器拉取最新代码。

#### 1. 检查本地仓库状态

```bash
git status -sb
git remote -v
git branch --show-current
```

确认：

- 当前分支是你准备部署的分支，例如 `main`
- 远程仓库指向你的 GitHub 仓库

#### 2. 如有需要，设置远程仓库地址

推荐使用 SSH：

```bash
git remote set-url origin git@github.com:xuseny/codexApi.git
```

验证本地是否能通过 SSH 访问 GitHub：

```bash
ssh -T git@github.com
```

#### 3. 提交本地修改

```bash
git add .
git commit -m "feat: 这里填写本次更新说明"
```

示例：

```bash
git commit -m "feat: 新增 API Key 单设备在线限制与兑换页踢下线展示"
```

#### 4. 推送到 GitHub

```bash
git push origin main
```

如果你的部署分支不是 `main`，请替换成实际分支名。

#### 5. 确认推送成功

```bash
git log -1 --oneline
```

然后在 GitHub 仓库页面确认最新提交已经出现，再继续后面的服务器拉取与部署步骤。

#### 6. 服务器部署时的关系

推荐流程如下：

```text
本地修改代码
  ↓
本地 git commit
  ↓
本地 git push 到 GitHub
  ↓
服务器 git clone / git pull
  ↓
服务器构建并替换二进制
```

### 3. 获取最新代码

建议使用一个临时源码目录，不直接在 `/opt/sub2api` 内操作。

首次拉取：

```bash
cd /root
git clone git@github.com:xuseny/codexApi.git
cd /root/codexApi
git log -1 --oneline
```

后续更新可以直接：

```bash
cd /root/codexApi
git pull
git log -1 --oneline
```

如果你希望每次都从干净目录开始，也可以：

```bash
cd /root
rm -rf /root/codexApi
git clone git@github.com:xuseny/codexApi.git
cd /root/codexApi
git log -1 --oneline
```

### 4. 安装前端依赖

```bash
cd /root/codexApi/frontend
pnpm install
```

### 5. 构建前端

```bash
cd /root/codexApi/frontend
pnpm run build
```

说明：

- 构建出现 chunk size warning 不影响继续部署
- 只要最后显示 `built` 即可继续

### 6. 编译后端二进制

必须使用 `embed`，否则前端构建结果不会被打进最终程序。

```bash
cd /root/codexApi/backend
go build -tags embed -o sub2api ./cmd/server
```

编译完成后检查文件：

```bash
ls -lh /root/codexApi/backend/sub2api
```

### 7. 停止线上服务

```bash
systemctl stop sub2api
```

### 8. 备份线上旧二进制

```bash
cp /opt/sub2api/sub2api /opt/sub2api/sub2api.bak-$(date +%Y%m%d-%H%M%S)
```

### 9. 用新二进制覆盖线上程序

```bash
cp /root/codexApi/backend/sub2api /opt/sub2api/sub2api
```

### 10. 修正权限

```bash
chown sub2api:sub2api /opt/sub2api/sub2api
chmod 755 /opt/sub2api/sub2api
```

### 11. 启动服务

```bash
systemctl start sub2api
```

### 12. 检查服务状态

```bash
systemctl status sub2api --no-pager
```

如果看到：

```text
Active: active (running)
```

说明服务已正常启动。

### 13. 查看最近日志

```bash
journalctl -u sub2api -n 80 --no-pager
```

如果日志中没有明显报错，且能看到正常请求日志，例如：

- `/api/v1/key-exchange/resolve`
- `/v1/responses`

通常说明新版本已经生效。

### 14. 浏览器验证

打开你的站点，例如：

```text
https://xuseny.online
https://api.xuseny.online
```

重点验证：

- 页面是否能正常打开
- 登录后台是否正常
- `Key 兑换页` 是否出现新功能
- API 调用是否正常

## 这套部署中不需要改动的内容

在“仅更新代码”的情况下，通常不需要修改以下文件：

- `/opt/sub2api/config.yaml`
- `/etc/caddy/Caddyfile`
- `/etc/systemd/system/sub2api.service`
- `/etc/systemd/system/sub2api.service.d/override.conf`

也不需要改动：

- PostgreSQL
- Redis
- Caddy 证书配置

## 回滚方法

如果新版本有问题，可直接回滚到上一个备份版本。

### 1. 停止服务

```bash
systemctl stop sub2api
```

### 2. 恢复备份二进制

将下面的文件名替换成实际备份名：

```bash
cp /opt/sub2api/sub2api.bak-20260330-011500 /opt/sub2api/sub2api
```

### 3. 修正权限

```bash
chown sub2api:sub2api /opt/sub2api/sub2api
chmod 755 /opt/sub2api/sub2api
```

### 4. 启动服务

```bash
systemctl start sub2api
```

### 5. 检查状态

```bash
systemctl status sub2api --no-pager
journalctl -u sub2api -n 80 --no-pager
```

## 常用检查命令

### 查看程序监听端口

```bash
ss -lntp | grep sub2api
```

### 查看服务定义

```bash
systemctl cat sub2api
```

### 查看配置文件

```bash
cat /opt/sub2api/config.yaml
```

### 查看 Caddy 配置

```bash
cat /etc/caddy/Caddyfile
```

### 查看 Redis 状态

```bash
systemctl status redis --no-pager
ps -ef | grep redis | grep -v grep
```

### 查看 PostgreSQL 连接情况

```bash
ps -ef | grep postgres | grep sub2api
```

## 一套最短命令版

如果你的服务器环境已经准备好，后续更新可以直接按下面整套执行：

```bash
cd /root
rm -rf /root/codexApi
git clone git@github.com:xuseny/codexApi.git
cd /root/codexApi/frontend
pnpm install
pnpm run build
cd /root/codexApi/backend
go build -tags embed -o sub2api ./cmd/server
systemctl stop sub2api
cp /opt/sub2api/sub2api /opt/sub2api/sub2api.bak-$(date +%Y%m%d-%H%M%S)
cp /root/codexApi/backend/sub2api /opt/sub2api/sub2api
chown sub2api:sub2api /opt/sub2api/sub2api
chmod 755 /opt/sub2api/sub2api
systemctl start sub2api
systemctl status sub2api --no-pager
journalctl -u sub2api -n 80 --no-pager
```

## 注意事项

### 1. 不要直接覆盖配置文件

不要用源码目录里的 `config.example.yaml` 去覆盖线上：

- `/opt/sub2api/config.yaml`

线上配置文件中包含真实的：

- 数据库地址
- 数据库密码
- Redis 配置
- JWT secret

### 2. 必须使用 embed 编译

错误示例：

```bash
go build -o sub2api ./cmd/server
```

正确示例：

```bash
go build -tags embed -o sub2api ./cmd/server
```

### 3. 如果只启动没替换，实际上还是旧版本

以下顺序是错误的：

```bash
systemctl stop sub2api
systemctl start sub2api
```

如果中间没有把新二进制复制到 `/opt/sub2api/sub2api`，那重启后仍然是旧版本。

### 4. 替换后二进制权限必须正确

如果不执行：

```bash
chown sub2api:sub2api /opt/sub2api/sub2api
chmod 755 /opt/sub2api/sub2api
```

可能导致 `systemd` 启动失败或权限异常。

## 适用范围说明

本文档仅适用于以下场景：

- 当前服务器使用的是二进制部署
- 不涉及数据库迁移冲突
- 只是把最新代码更新到现有应用

如果你后续要改成 Docker 部署，请单独整理迁移方案，不建议把当前二进制部署与 Docker 直接混用。
