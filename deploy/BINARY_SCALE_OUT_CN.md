# Sub2API 二进制双机扩容文档

本文档适用于以下现状：

- 你已经有 1 台正在运行的二进制服务器
- 当前部署方式是 `systemd + Caddy + PostgreSQL + Redis`
- 你现在新增了第 2 台服务器，希望用来做横向扩容
- 你希望继续沿用原来的 1 个域名，不改成 Docker/K8s

本文档按下面这个最小改动方案展开。

说明：

- 本文中的“旧服务器 A / 主节点 A”是角色代号，表示“当前主节点”
- 截至 2026-04-10，`xuseny.online` 现网主节点已经迁移到 `154.64.230.47`
- 旧 `156.239.40.153` 如果仍在线，默认只应视为 DNS / HTTPS 证书传播期间的临时代理或回滚机
- 因此阅读本文时，请不要再机械地把“历史上的 A 机公网 IP”理解成当前 PostgreSQL / Redis 所在主机

```text
公网域名 xuseny.online
  ↓
旧服务器 A（公网入口机）
  - Caddy
  - sub2api 节点 1
  - PostgreSQL
  - Redis
  - datamanagementd（如已启用）
  ↓ 负载均衡
新服务器 B（应用扩容机）
  - sub2api 节点 2
```

## 1. 这套方案的核心思路

你不需要多个域名。

继续保持：

- 域名只解析到旧服务器 A
- A 上的 Caddy 继续做统一入口
- Caddy 把请求分发到：
  - A 本机的 `sub2api`
  - B 上的 `sub2api`

这样用户看到的仍然只有一个域名。

## 2. 推荐角色划分

### 旧服务器 A

保留以下角色：

- 公网入口
- Caddy
- `sub2api` 节点 1
- PostgreSQL
- Redis
- `datamanagementd`（如果你已经启用“数据管理”）

### 新服务器 B

只承担：

- `sub2api` 节点 2

不要在 B 上重复部署：

- Caddy
- PostgreSQL
- Redis

这样改动最小，回滚也最简单。

## 3. 先确认两台机器的信息

下面文档按一套最常见的“两台机器 + 一个域名”场景展开：

- 主节点 A 地址：`156.239.40.153`
- 应用节点 B 地址：`156.239.47.99`
- 站点域名：`xuseny.online`
- `sub2api` 监听端口：`2127`

注意：

- 这两个地址从格式上看，不是常见的 RFC1918 私网地址
- 下面我先按你提供的地址直接写可执行示例
- 如果两台机器实际上还有真正的内网地址，例如 `10.x.x.x`、`172.16-31.x.x`、`192.168.x.x`，请优先改成真正内网地址
- 如果最终仍然只能走这两个地址通信，务必用防火墙或安全组把 `5432`、`6379`、`2127` 锁死到对端 IP，不能裸露到公网

如果你已经完成过“主节点迁移”，这里的 `A` 应替换成你当前真正承载 PostgreSQL / Redis / Caddy / 主应用的那台机器，而不是历史上的旧公网入口机。

## 4. 扩容前必须知道的 3 个限制

### 1. PostgreSQL 和 Redis 必须允许 B 访问

当前单机部署里，它们大概率只监听 `localhost`。

扩容后要改成：

- A 自己还能访问
- B 也能通过 A 的内网 IP 访问
- 只对 B 放行，不对公网开放

### 2. `datamanagementd` 默认仍然只放在 A

`datamanagementd` 依赖本机 Unix Socket：

- `/tmp/sub2api-datamanagement.sock`

而且元数据默认是本机 SQLite。

所以最省事的方式是：

- 仍然只在 A 上跑 `datamanagementd`
- `/api/v1/admin/data-management/*` 相关请求固定回 A




或二进制部署对应的本地目录。

多机后会出现：

- 文件在 A 上生成
- 请求被 Caddy 转发到 B
- B 本地没有这个文件

所以二选一：


本文档默认采用第 2 种过渡方案。

## 5. 旧服务器 A：放通 PostgreSQL 和 Redis 给 B

### 5.1 PostgreSQL

先查看 PostgreSQL 配置文件位置：

```bash
sudo -u postgres psql -c "SHOW config_file;"
sudo -u postgres psql -c "SHOW hba_file;"
```

常见位置例如：

- `/etc/postgresql/16/main/postgresql.conf`
- `/etc/postgresql/16/main/pg_hba.conf`

修改 `postgresql.conf`：

```conf
listen_addresses = '127.0.0.1,156.239.40.153'
```

修改 `pg_hba.conf`，追加一条只允许 B 访问：

```conf
host    sub2api    sub2api    156.239.47.99/32    scram-sha-256
```

如果你的数据库名或用户名不是 `sub2api`，替换成你线上实际值。

重启 PostgreSQL：

```bash
sudo systemctl restart postgresql
sudo systemctl status postgresql --no-pager
```

### 5.2 Redis

先查看 Redis 配置位置，常见是：

- `/etc/redis/redis.conf`
- `/etc/redis/redis-server.conf`

修改为：

```conf
bind 127.0.0.1 156.239.40.153
protected-mode yes
port 6379
requirepass 你的现有Redis密码
```

如果你当前已经在用 `requirepass`，不要改密码，保持与线上一致即可。

重启 Redis：

```bash
sudo systemctl restart redis
sudo systemctl status redis --no-pager
```

## 6. 防火墙 / 安全组建议

### 旧服务器 A 放行

只允许新服务器 B 访问：

- `5432/tcp`
- `6379/tcp`

### 新服务器 B 放行

只允许旧服务器 A 访问：

- `2127/tcp`

### 公网暴露

仍然只保留旧服务器 A 暴露：

- `80/tcp`
- `443/tcp`

新服务器 B 不需要对公网暴露站点端口。

## 7. 新服务器 B：部署第二个 sub2api 节点

最省事的做法不是重新手搓全部配置，而是直接从 A 复制：

- 二进制
- 配置文件
- systemd 服务文件

### 7.1 在 B 上准备目录和用户

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin sub2api || true
sudo mkdir -p /opt/sub2api
sudo chown -R sub2api:sub2api /opt/sub2api
```

### 7.2 从 A 复制二进制和配置

在 A 上执行：

```bash
scp /opt/sub2api/sub2api root@156.239.47.99:/opt/sub2api/
scp /opt/sub2api/config.yaml root@156.239.47.99:/opt/sub2api/
scp /etc/systemd/system/sub2api.service root@156.239.47.99:/etc/systemd/system/
```

如果你的配置不在 `/opt/sub2api/config.yaml`，按你的实际路径复制。

### 7.3 在 B 上修改配置

打开：

```bash
sudo nano /opt/sub2api/config.yaml
```

重点确认以下字段：

#### 数据库改成连接 A

```yaml
database:
  host: "156.239.40.153"
  port: 5432
```

#### Redis 改成连接 A

```yaml
redis:
  host: "156.239.40.153"
  port: 6379
```

#### 前端域名保持一致

```yaml
server:
  frontend_url: "https://xuseny.online"
```

#### JWT / TOTP 密钥必须与 A 保持一致

这两个值绝对不能和 A 不一致：

- `jwt.secret`
- `totp.encryption_key`

如果你是直接从 A 复制配置文件过来的，这两项通常已经一致。

### 7.4 在 B 上设置监听地址

因为 B 不直接对公网提供服务，所以只需要给 A 访问即可。

如果你用私网转发，推荐：

```ini
[Service]
Environment=SERVER_HOST=0.0.0.0
Environment=SERVER_PORT=2127
```

执行：

```bash
sudo systemctl daemon-reload
sudo systemctl edit sub2api
```

填入：

```ini
[Service]
Environment=SERVER_HOST=0.0.0.0
Environment=SERVER_PORT=2127
```

保存后启动：

```bash
sudo chown sub2api:sub2api /opt/sub2api/sub2api
sudo chmod 755 /opt/sub2api/sub2api
sudo systemctl enable --now sub2api
sudo systemctl status sub2api --no-pager
```

### 7.5 从 B 测试到 A 的数据库和 Redis

先确认端口连通：

```bash
nc -vz 156.239.40.153 5432
nc -vz 156.239.40.153 6379
```

再看应用日志：

```bash
sudo journalctl -u sub2api -n 80 --no-pager
```

如果能正常启动，说明第二个节点已经具备接流能力。

## 8. 旧服务器 A：把 Caddy 改成双节点负载均衡

你只需要修改 A 上的 Caddy，不需要改 DNS。

参考仓库里的示例文件：

- `deploy/Caddyfile.binary-scaleout.example`
- `deploy/Caddyfile.xuseny.online.scaleout`

核心逻辑是：

- 普通请求：在 A 和 B 之间轮询
- `/api/v1/admin/data-management/*`：固定回 A

这样能避免本地媒体和 `datamanagementd` 跨机不一致的问题。

替换后执行：

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
sudo systemctl status caddy --no-pager
```

## 9. 双机扩容后的推荐拓扑

```text
xuseny.online
  ↓
旧服务器 A / Caddy
  ├─ 普通 API / 页面请求 -> A:2127 + B:2127
  └─ /api/v1/admin/data-management/* -> A:2127

旧服务器 A
  ├─ sub2api 节点 1
  ├─ PostgreSQL
  ├─ Redis
  └─ datamanagementd

新服务器 B
  └─ sub2api 节点 2
```

## 10. 扩容完成后的验证

### 10.1 检查 B 上服务是否正常

```bash
sudo systemctl status sub2api --no-pager
sudo journalctl -u sub2api -n 80 --no-pager
ss -lntp | grep 2127
```

### 10.2 检查 Caddy 配置是否生效

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo journalctl -u caddy -n 80 --no-pager
```

### 10.3 实际打流量观察

在 A、B 两台机器分别开日志：

```bash
sudo journalctl -u sub2api -f
```

然后从本地反复访问你的站点或跑接口请求。

如果两边都能收到请求，说明负载均衡已经生效。

## 11. 回滚方法

如果扩容后有问题，最快的回滚方式是：

### 1. 在 A 的 Caddy 中移除 B

把 B 的上游地址从 `reverse_proxy` 里删掉，只保留 A 本机。

### 2. 重载 Caddy

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

### 3. 停掉 B 上的应用

```bash
sudo systemctl stop sub2api
```

这样整个站点会立即回到原来的单机模式。

## 12. 后续进一步扩容时怎么做

如果以后再加第 3 台、第 4 台服务器，继续照这个模式扩：

- 域名仍然只指向入口机 A
- 新增服务器只部署 `sub2api`
- 所有节点共用同一个 PostgreSQL 和 Redis
- Caddy 继续追加上游节点

但要注意：

- 应用节点越多，数据库连接数和 Redis 连接池会放大
- 默认配置下每台应用机连接数并不低
- 当节点继续增加时，应优先考虑：
  - 降低每节点数据库连接池
  - 给 PostgreSQL 增加连接池中间层，例如 `pgbouncer`

## 13. 一句话建议

对于你现在“新增 1 台服务器”的场景，最稳妥的落地方式就是：

- A 继续当入口机 + 数据机
- B 只跑第二个应用节点
- 1 个域名不变
- 通过 Caddy 做双节点负载均衡

这样是改动最小、上线最快、出问题也最好回滚的方案。
