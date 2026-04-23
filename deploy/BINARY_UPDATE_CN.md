# Sub2API 二进制部署更新文档

本文档适用于已经使用以下方式部署的服务器：

- `Sub2API` 以二进制形式运行
- 由 `systemd` 管理服务
- 使用 `Caddy` 反向代理
- `PostgreSQL` 与 `Redis` 运行在宿主机

如果你现在已经新增了第 2 台服务器，准备做横向扩容，请优先参考：

- `deploy/BINARY_SCALE_OUT_CN.md`

截至 2026-04-10，`xuseny.online` 的主节点已经迁移到新服务器 `154.64.230.47`。如果你线上还保留着旧服务器 `156.239.40.153`，默认应只把它当作 DNS / HTTPS 证书传播期间的临时反代或回滚机，不要再把它当成长期主节点执行常规更新。

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

补充说明：

- 当前生产环境推荐由 `sub2api` 只监听 `127.0.0.1:2127`
- 对外统一由 `Caddy` 提供 `80/443`
- 如果你在迁移主节点后的过渡期仍保留旧公网入口，旧入口机应只做反代，不应继续承载主数据库和主应用写流量

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


### 可选：B 机快速更新命令

如果当前线上已经是“主节点 + 应用节点”的双机结构，也可以在 B 机直接使用一条命令完成更新：

```bash
cd /root && rm -rf codexApi && git clone git@github.com:xuseny/codexApi.git && cd /root/codexApi/frontend && pnpm install && pnpm run build && cd /root/codexApi/backend && go build -tags embed -o sub2api ./cmd/server && systemctl stop sub2api && cp /opt/sub2api/sub2api /opt/sub2api/sub2api.bak-$(date +%Y%m%d-%H%M%S) && cp /root/codexApi/backend/sub2api /opt/sub2api/sub2api && chown sub2api:sub2api /opt/sub2api/sub2api && chmod 755 /opt/sub2api/sub2api && systemctl start sub2api && systemctl status sub2api --no-pager && journalctl -u sub2api -n 80 --no-pager -l
```

注意：

- 这条命令只适用于纯应用节点 B
- 这条命令默认 B 机已经完成首次部署，且已具备 `git`、`go`、`node`、`pnpm`、`sub2api` systemd 服务、`sub2api` 用户和 `/opt/sub2api` 目录
- 如果 B 是一台全新空白机器，请先按 `deploy/BINARY_SCALE_OUT_CN.md` 完成初始化部署，再使用这条命令做后续更新
- B 的数据库 / Redis 配置应始终指向当前主节点
- 不要在旧过渡入口机上误执行这条命令

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

### 5. 不要用 `root` 手工运行 `caddy run` 代替 systemd

排查 HTTPS / 证书问题时，一个非常容易误判的点是：

- `systemctl start caddy` 使用的是 systemd 下 `caddy` 用户的存储目录
- `root` 手工执行 `caddy run --config /etc/caddy/Caddyfile` 使用的是 `root` 自己的 Home 目录

这会导致你看到两套不同的证书缓存与 ACME 状态。

线上排查时优先使用：

```bash
systemctl status caddy --no-pager -l
journalctl -u caddy -n 100 --no-pager
find /var/lib/caddy/.local/share/caddy/certificates -type f
```

不要把 `root` 前台运行的 Caddy 状态直接当成最终线上状态。

### 6. 主节点迁移后，如 DNS / ACME 传播未稳定，可暂时保留旧入口机反代到新主机

如果你已经把 DNS 改到新主机，但仍发现：

- 有些地区还能访问旧公网 IP
- Let's Encrypt 校验还在命中旧公网 IP
- 新主机 HTTPS 证书一时签不下来

最稳妥的过渡方式是：

- 新主机正常运行 `sub2api + Caddy`
- 旧入口机停止 `sub2api`
- 旧入口机仅保留 `Caddy`
- 旧入口机把 `80/443` 请求反代到 `http://新主机:80`

这样既能保证业务不中断，也能让 ACME 校验最终命中新主机。

如果你希望立即恢复新主机 HTTPS，也可以直接把旧主机上可用的 Caddy 证书状态目录同步到新主机：

```text
/var/lib/caddy/.local/share/caddy/certificates
```

## Docker 版部署与更新命令

以下内容适用于当前这套已经在线验证过的结构：

- 主机 `154.64.230.47`：`Caddy + PostgreSQL + Redis + sub2api Docker`
- 一号机 `154.64.230.156`：`sub2api Docker`
- `Caddy` 仍只放在主机，应用层双节点分别是 `127.0.0.1:2127` 和 `154.64.230.156:2127`

这不是 `deploy/README.md` 里的全栈 Docker Compose，而是“宿主机保留 PostgreSQL / Redis / Caddy，只把 `sub2api` 改成 Docker 运行”的方案。

### 主机：首次切到 Docker

先安装 Docker：

```bash
apt update
apt install -y docker.io docker-compose-v2 git
systemctl enable --now docker
docker compose version
docker version
```

准备代码和运行目录：

```bash
cd /root
rm -rf /root/codexApi
git clone git@github.com:xuseny/codexApi.git
cd /root/codexApi
git log -1 --oneline

mkdir -p /opt/sub2api-docker/data/logs
cp /opt/sub2api/config.yaml /opt/sub2api-docker/config.yaml
```

主机的 `/opt/sub2api-docker/config.yaml` 建议至少确认：

```yaml
server:
  trusted_proxies:
    - "127.0.0.1/32"
    - "::1/128"
```

写入主机用的 `docker-compose.yml`：

```bash
cat > /opt/sub2api-docker/docker-compose.yml <<'EOF'
services:
  sub2api:
    build:
      context: /root/codexApi
      dockerfile: deploy/Dockerfile
    image: codexapi:local
    container_name: sub2api
    restart: unless-stopped
    network_mode: host
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/data/config.yaml:ro
    environment:
      - AUTO_SETUP=false
      - TZ=Asia/Shanghai
EOF
```

修权限并切换：

```bash
chown 1000:1000 /opt/sub2api-docker/config.yaml
chmod 600 /opt/sub2api-docker/config.yaml
chown -R 1000:1000 /opt/sub2api-docker/data
chmod 755 /opt/sub2api-docker/data /opt/sub2api-docker/data/logs

cd /opt/sub2api-docker
docker compose build sub2api

systemctl stop sub2api
docker compose up -d sub2api
docker compose ps
docker compose logs --tail=100 sub2api
curl http://127.0.0.1:2127/health

systemctl disable sub2api
```

### 主机：后续更新命令

主机后续更新直接用这一套：

```bash
cd /root/codexApi
git pull
git log -1 --oneline

cd /opt/sub2api-docker
docker compose build sub2api
docker compose up -d sub2api
docker compose ps
docker compose logs --tail=100 sub2api
curl http://127.0.0.1:2127/health
```

### 一号机：首次切到 Docker

先安装 Docker：

```bash
apt update
apt install -y docker.io docker-compose-v2 git redis-tools
systemctl enable --now docker
docker compose version
docker version
```

准备代码和运行目录：

```bash
cd /root
rm -rf /root/codexApi
git clone git@github.com:xuseny/codexApi.git
cd /root/codexApi
git log -1 --oneline

mkdir -p /opt/sub2api-docker/data/logs
cp /opt/sub2api/config.yaml /opt/sub2api-docker/config.yaml
```

一号机的 `/opt/sub2api-docker/config.yaml` 必须确认：

```yaml
server:
  trusted_proxies:
    - "154.64.230.47/32"
  host: 0.0.0.0
  port: 2127

database:
  host: 154.64.230.47

redis:
  host: 154.64.230.47
```

写入一号机用的 `docker-compose.yml`：

```bash
cat > /opt/sub2api-docker/docker-compose.yml <<'EOF'
services:
  sub2api:
    build:
      context: /root/codexApi
      dockerfile: deploy/Dockerfile
    image: codexapi:local
    container_name: sub2api
    restart: unless-stopped
    network_mode: host
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/data/config.yaml:ro
    environment:
      - AUTO_SETUP=false
      - TZ=Asia/Shanghai
      - SERVER_HOST=0.0.0.0
      - SERVER_PORT=2127
EOF
```

修权限并切换：

```bash
chown 1000:1000 /opt/sub2api-docker/config.yaml
chmod 600 /opt/sub2api-docker/config.yaml
chown -R 1000:1000 /opt/sub2api-docker/data
chmod 755 /opt/sub2api-docker/data /opt/sub2api-docker/data/logs

cd /opt/sub2api-docker
docker compose build sub2api

systemctl stop sub2api
docker compose up -d sub2api
docker compose ps
docker compose logs --tail=100 sub2api
curl http://127.0.0.1:2127/health

systemctl disable sub2api
```

如果一号机需要先确认主机 Redis 可连通，可执行：

```bash
redis-cli -h 154.64.230.47 -p 6379 -a '你的 Redis 密码' ping
```

### 一号机：后续更新命令

一号机后续更新直接用这一套：

```bash
cd /root/codexApi
git pull origin main

cd /opt/sub2api-docker
docker compose build sub2api
docker compose up -d --force-recreate sub2api

docker compose ps
docker compose logs --tail=100 sub2api
curl http://127.0.0.1:2127/health

```

### Docker 版回滚命令

如果 Docker 版有问题，需要临时回滚到旧 `systemd` 二进制：

```bash
cd /opt/sub2api-docker
docker compose down

systemctl start sub2api
systemctl enable sub2api
systemctl status sub2api --no-pager -l
```

## 适用范围说明

本文档包含两类场景：

- 前半部分：二进制部署更新
- 上一章：宿主机保留 PostgreSQL / Redis / Caddy，仅把 `sub2api` 改成 Docker 运行

共同前提：

- 不涉及数据库迁移冲突
- 只是把最新代码更新到现有应用

如果你使用的是 `deploy/README.md` 里的全栈 Docker Compose，请优先参考 `deploy/README.md`，不要把本文的二进制流程与全栈 Docker 流程直接混用。
