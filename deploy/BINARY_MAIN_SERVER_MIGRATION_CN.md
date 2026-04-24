# Sub2API 主服务器迁移文档

本文档用于把当前承担 `Caddy + PostgreSQL + Redis + sub2api` 的旧主服务器，迁移到一台性能更好的新服务器上。

本文默认参考并复用以下文档中的约定：

- `deploy/BINARY_UPDATE_CN.md`
- `deploy/BINARY_SCALE_OUT_CN.md`
- `deploy/服务器扩容手册.md`

## 适用场景

- 当前部署方式为 `systemd + Caddy + PostgreSQL + Redis + sub2api`
- `sub2api` 以二进制方式运行
- 当前旧主服务器 A 承担：
  - 公网入口
  - Caddy
  - PostgreSQL
  - Redis
  - `sub2api`
  - `datamanagementd`（如已启用）
- 你准备把这些“主节点职责”整体迁移到新服务器 C
- 当前可能还存在应用节点 B；如果没有，忽略本文所有 B 相关步骤

## 本文默认前提

- 继续沿用原有域名，不改为 Docker / K8s
- 可以接受一次“短停机窗口”做最终切换
- 旧主服务器 A 在迁移完成后暂不销毁，先作为回滚兜底保留
- 本文沿用现网手册常见路径：`/opt/sub2api/config.yaml`

如果你的实际配置路径不是 `/opt/sub2api/config.yaml`，请始终以你线上真实路径为准。

如果你的数据库体量很大、停机窗口非常短，本文中的 `pg_dump / pg_restore` 只适合作为基线方案；这类场景更建议提前做 PostgreSQL 主从或逻辑复制，不建议直接照搬本文的最终切换步骤。

## 0. 实际迁移补充（2026-04-10）

`xuseny.online` 在 2026-04-09 至 2026-04-10 的实际主节点迁移过程中，额外踩到了下面几个非常重要的坑，建议在正式执行前先看完：

### 0.1 不要用 `root` 前台运行 `caddy run` 代替 systemd

迁移时如果你用：

```bash
caddy run --config /etc/caddy/Caddyfile
```

它会使用 `root` 自己的 Home 和证书存储目录；而线上真正的：

```bash
systemctl start caddy
```

使用的是 systemd 下 `caddy` 用户的存储目录。两者不是同一套证书状态。

实际排查时应优先看：

```bash
systemctl status caddy --no-pager -l
journalctl -u caddy -n 100 --no-pager
find /var/lib/caddy/.local/share/caddy/certificates -type f
```

### 0.2 即使 DNS 已切到新主机，ACME 校验流量仍可能命中旧公网 IP

本次迁移中已经确认：

- 域名面板里的 A 记录已经改到新主机
- 但 Let's Encrypt 在一段时间内仍然访问到了旧公网 IP

这会导致新主机上直接申请 HTTPS 证书失败。

### 0.3 最有效的过渡方式：旧 A 暂时只做 HTTPS 反代到新 C

如果 DNS / ACME 传播还没稳定，最有效的临时方案是：

- 新主机 C 正常运行 `sub2api + Caddy`
- 旧主机 A 停掉 `sub2api`
- 旧主机 A 仅保留 `Caddy`
- 旧主机 A 把 `80/443` 请求反代到 `http://C_NEW_IP:80`

这样可以保证：

- 命中旧公网 IP 的用户请求仍能进入新主机
- 命中旧公网 IP 的 ACME HTTP/TLS 校验也能最终到达新主机

### 0.4 想立即恢复新主机 HTTPS 时，可直接同步旧主机的 Caddy 证书状态

本次迁移中，旧主机 A 上已有可用证书时，可以直接同步下面目录到新主机 C：

```text
/var/lib/caddy/.local/share/caddy/certificates
```

同步后记得修正：

```bash
chown -R caddy:caddy /var/lib/caddy
```

## 1. 迁移后的目标拓扑

迁移前：

```text
域名
  -> 旧主服务器 A / Caddy
      -> A 本机 sub2api
      -> B 应用节点（如存在）

旧主服务器 A
  - Caddy
  - PostgreSQL
  - Redis
  - sub2api
  - datamanagementd（如启用）

应用节点 B（可选）
  - sub2api
```

迁移后：

```text
域名
  -> 新主服务器 C / Caddy
      -> C 本机 sub2api
      -> B 应用节点（如保留）

新主服务器 C
  - Caddy
  - PostgreSQL
  - Redis
  - sub2api
  - datamanagementd（如启用）

应用节点 B（可选）
  - sub2api

旧主服务器 A
  - 迁移完成后保留一段时间，仅作回滚兜底
```

## 2. 迁移原则

### 2.1 不要直接“停旧机再手搓新机”

最稳妥的顺序是：

1. 先把新服务器 C 预装好
2. 先把应用和配置同步到 C
3. 先做一次非生产流量验活
4. 再进入短停机窗口做最终数据切换
5. 最后切 DNS / 公网入口

### 2.2 共享密钥绝对不能变

以下值在迁移前后必须保持一致：

- `jwt.secret`
- `redis.password`
- `totp.encryption_key`

否则会出现：

- 登录态失效
- 跨节点行为异常
- TOTP/2FA 无法互通


不要只迁移数据库。

如果你使用了以下能力，也要一起迁移：

- `datamanagementd` SQLite 元数据
  - 默认：`/var/lib/sub2api/datamanagement/datamanagementd.db`

### 2.4 数据层迁移完成前，不要让新旧主同时承接写流量

迁移窗口里必须避免 A 和 C 同时写 PostgreSQL / Redis / 本地文件，否则会出现双写分叉，后面很难收敛。

## 3. 变量约定

下文用以下占位符描述步骤，请替换成你的真实值：

- `A_OLD_IP`：旧主服务器 A 的内网 IP
- `B_APP_IP`：应用节点 B 的内网 IP（没有就忽略）
- `C_NEW_IP`：新主服务器 C 的内网 IP
- `DOMAIN`：主域名
- `PG_DB`：PostgreSQL 数据库名
- `PG_USER`：PostgreSQL 用户名
- `REDIS_PASSWORD`：Redis 当前线上密码

强烈建议优先使用真实内网 IP，不要用公网/NAT 映射地址作为节点间通信地址。

如果你现在只有公网地址可用，务必用安全组或防火墙把 `5432`、`6379`、`2127` 限制到指定对端 IP。

## 4. 迁移前检查

### 4.1 先从旧主服务器 A 导出现网事实

在 A 上至少确认：

```bash
systemctl status sub2api postgresql redis caddy --no-pager
systemctl cat sub2api
test -f /etc/systemd/system/sub2api.service.d/override.conf && cat /etc/systemd/system/sub2api.service.d/override.conf
cat /opt/sub2api/config.yaml
cat /etc/caddy/Caddyfile
sudo -u postgres psql -c "SHOW server_version;"
redis-server --version
```

如果你启用了 `datamanagementd`，额外确认：

```bash
systemctl status sub2api-datamanagementd --no-pager
systemctl cat sub2api-datamanagementd
ls -lh /var/lib/sub2api/datamanagement/
```

建议记录以下内容：

- PostgreSQL 版本
- Redis 版本
- Caddy 配置
- `sub2api.service`
- `override.conf` 中的环境变量
- `config.yaml`
- `datamanagementd` SQLite 路径

### 4.2 提前处理 DNS TTL

如果最终要靠 DNS 把域名切到 C，建议至少提前几小时把 TTL 降到 `300` 秒或更低。

### 4.3 提前确认 Caddy 证书/数据目录

如果你希望新主机 C 切流后尽量不等待重新签证书，建议在 A 上先确认 Caddy 的数据目录：

```bash
caddy environ
systemctl cat caddy
```

然后记录实际的：

- Caddy 数据目录
- Caddy 配置目录
- 证书/ACME 存储目录

后面可以选择一并同步到 C。

如果你不迁移 Caddy 的证书/数据目录，也可以在切流后让 C 重新签发证书，只是首次公网切换时要预留一点证书签发时间。

如果你希望在切流后尽快恢复 HTTPS，建议直接提前备份旧主机上的：

```text
/var/lib/caddy/.local/share/caddy/certificates
```

这比在切换后临时手工 `root` 运行 `caddy run` 更可靠。

### 4.4 提前确认 B 是否继续保留

如果迁移后 B 仍作为应用节点保留，则迁移完成后要同时修改 B 的：

- `database.host`
- `database.port`
- `redis.host`
- `redis.port`

全部改到新主机 C。

## 5. 第一阶段：先把新主服务器 C 预装好

### 5.1 安装与 A 相同或兼容的基础环境

C 上至少要准备：

- `sub2api` 运行用户
- `/opt/sub2api`
- PostgreSQL
- Redis
- Caddy
- systemd

如果你准备“迁移时顺带升级代码”，可以按 `deploy/BINARY_UPDATE_CN.md` 的方式重新构建二进制；如果你这次只做迁移，不改版本，直接从 A 复制现网二进制最稳。

### 5.2 在 C 上准备目录和用户

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin sub2api || true
sudo mkdir -p /opt/sub2api
sudo chown -R sub2api:sub2api /opt/sub2api
```

如果启用了 `datamanagementd`，再准备：

```bash
sudo mkdir -p /var/lib/sub2api/datamanagement
sudo chown -R sub2api:sub2api /var/lib/sub2api/datamanagement
```

### 5.3 从 A 同步程序和配置到 C

在 A 上执行：

```bash
scp /opt/sub2api/sub2api root@C_NEW_IP:/opt/sub2api/
scp /opt/sub2api/config.yaml root@C_NEW_IP:/opt/sub2api/
scp /etc/systemd/system/sub2api.service root@C_NEW_IP:/etc/systemd/system/
test -d /etc/systemd/system/sub2api.service.d && scp -r /etc/systemd/system/sub2api.service.d root@C_NEW_IP:/etc/systemd/system/
```

如果启用了 `datamanagementd`，额外同步：

```bash
scp /opt/sub2api/datamanagementd root@C_NEW_IP:/opt/sub2api/
scp /etc/systemd/system/sub2api-datamanagementd.service root@C_NEW_IP:/etc/systemd/system/
```

### 5.4 推荐先把 C 当作“临时应用节点”验活一次

这是整次迁移里最值的预检查。

在 C 的 `/opt/sub2api/config.yaml` 里，先让它暂时连接旧主机 A：

```yaml
server:
  host: "0.0.0.0"
  port: 2127

database:
  host: "A_OLD_IP"
  port: 5432

redis:
  host: "A_OLD_IP"
  port: 6379
```

其余共享密钥保持和 A 完全一致，不要改。

如果 `sub2api.service` 默认不是监听 `2127`，就通过 `override.conf` 或 systemd override 改成：

```ini
[Service]
Environment=SERVER_HOST=0.0.0.0
Environment=SERVER_PORT=2127
```

然后在 C 上启动并检查：

```bash
sudo systemctl daemon-reload
sudo chown sub2api:sub2api /opt/sub2api/sub2api /opt/sub2api/config.yaml
sudo chmod 755 /opt/sub2api/sub2api
sudo chmod 640 /opt/sub2api/config.yaml
sudo systemctl enable --now sub2api
sudo systemctl status sub2api --no-pager
curl http://127.0.0.1:2127/health
```

如果这一步都过不去，先不要进入主节点迁移窗口。

## 6. 第二阶段：先做一次“预同步”

目的不是立刻切流，而是先把大头数据搬到 C，减少最终停机窗口。

### 6.1 PostgreSQL 预同步

建议从 `config.yaml` 取出线上实际的 `PG_DB` / `PG_USER`，不要想当然写死。

在 A 上先导出角色和数据库：

```bash
sudo -u postgres pg_dumpall --globals-only > /root/sub2api-globals.sql
sudo -u postgres pg_dump -Fc -d PG_DB > /root/sub2api-precutover.dump
scp /root/sub2api-globals.sql root@C_NEW_IP:/root/
scp /root/sub2api-precutover.dump root@C_NEW_IP:/root/
```

在 C 上导入：

```bash
sudo -u postgres psql -f /root/sub2api-globals.sql
sudo -u postgres createdb -O PG_USER PG_DB || true
sudo -u postgres pg_restore -d PG_DB --clean --if-exists /root/sub2api-precutover.dump
```

### 6.2 Redis 预同步

如果你希望尽量保留 Redis 里的登录态、缓存和运行态数据，可以先做一次预同步。

在 A 上：

```bash
redis-cli -a 'REDIS_PASSWORD' BGSAVE
scp /var/lib/redis/dump.rdb root@C_NEW_IP:/var/lib/redis/
```

如果你的 Redis 开启了 AOF，请同步实际的 AOF 目录，而不只同步 `dump.rdb`。

如果你能接受 Redis 运行态数据在切换时丢失，也可以只保留同一个 `redis.password`，在最终切换时不迁移 Redis 文件。



```text
```

先在 A 上查清真实目录，再同步：

```bash
```

### 6.4 datamanagementd 元数据预同步

如果启用了 `datamanagementd`，同步 SQLite 目录：

```bash
rsync -aHAX --info=progress2 /var/lib/sub2api/datamanagement/ root@C_NEW_IP:/var/lib/sub2api/datamanagement/
```

### 6.5 Caddy 配置与数据预同步

配置文件建议先同步：

```bash
scp /etc/caddy/Caddyfile root@C_NEW_IP:/etc/caddy/
```

如果你确认了 Caddy 的证书/数据目录，也可以一并同步到 C。

## 7. 第三阶段：进入停机窗口做最终切换

### 7.1 先停掉所有会写主数据的 `sub2api`

先停 A：

```bash
sudo systemctl stop sub2api
```

如果 B 仍在跑，也要停：

```bash
sudo systemctl stop sub2api
```

这一步之后，数据库和 Redis 理论上不应再有新的业务写入。

### 7.2 做 PostgreSQL 最终同步

在 A 上重新导出一份最终 dump：

```bash
sudo -u postgres pg_dump -Fc -d PG_DB > /root/sub2api-cutover.dump
scp /root/sub2api-cutover.dump root@C_NEW_IP:/root/
```

在 C 上覆盖恢复：

```bash
sudo -u postgres pg_restore -d PG_DB --clean --if-exists /root/sub2api-cutover.dump
```

### 7.3 做 Redis、本地文件、datamanagementd 的最终同步

如果你要保留 Redis 运行态数据：

在 A 上：

```bash
redis-cli -a 'REDIS_PASSWORD' SAVE
sudo systemctl stop redis
rsync -aHAX /var/lib/redis/ root@C_NEW_IP:/var/lib/redis/
```

同步 Redis 文件前，确保 C 上的 `redis` 处于停止状态。

然后同步本地文件：

```bash
rsync -aHAX --delete /var/lib/sub2api/datamanagement/ root@C_NEW_IP:/var/lib/sub2api/datamanagement/
```

如果启用了 `datamanagementd`，同步 SQLite 目录前也要确保 C 上的 `sub2api-datamanagementd` 已停止。

建议在最终切换前把 A 上的 PostgreSQL 也停掉，避免旧主机被误用：

```bash
sudo systemctl stop postgresql
```

### 7.4 把 C 的配置改成“真正的新主机模式”

在 C 的 `/opt/sub2api/config.yaml` 中，把数据库和 Redis 改回本机：

```yaml
database:
  host: "127.0.0.1"
  port: 5432

redis:
  host: "127.0.0.1"
  port: 6379
```

同时确认以下值与旧主机 A 保持一致：

- `server.frontend_url`
- `jwt.secret`
- `redis.password`
- `totp.encryption_key`

如果你保留 B，则把 B 的 `/opt/sub2api/config.yaml` 改到新主机 C：

```yaml
database:
  host: "C_NEW_IP"
  port: 5432

redis:
  host: "C_NEW_IP"
  port: 6379
```

### 7.5 配置 C 上 PostgreSQL / Redis 的监听

如果迁移后还保留 B，则 C 上 PostgreSQL 需要允许 B 访问。

PostgreSQL 建议：

```conf
listen_addresses = '127.0.0.1,C_NEW_IP'
```

`pg_hba.conf` 增加：

```conf
host    PG_DB    PG_USER    B_APP_IP/32    scram-sha-256
```

Redis 建议：

```conf
bind 127.0.0.1 C_NEW_IP
protected-mode yes
port 6379
requirepass REDIS_PASSWORD
```

如果迁移后不再保留 B，则 PostgreSQL / Redis 都可以只监听本机。

### 7.6 在 C 上启动顺序

在 C 上按顺序启动：

```bash
sudo systemctl daemon-reload
sudo systemctl start postgresql
sudo systemctl start redis
sudo systemctl start sub2api-datamanagementd   # 如启用
sudo systemctl start sub2api
sudo systemctl start caddy
```

检查：

```bash
sudo systemctl status postgresql redis sub2api caddy --no-pager
curl http://127.0.0.1:2127/health
```

如果保留 B，确认 C 就绪后，再在 B 上启动：

```bash
sudo systemctl restart sub2api
sudo systemctl status sub2api --no-pager
```

## 8. 切换公网入口

### 8.1 如果你用的是浮动 IP / EIP

最简单，直接把浮动 IP 绑到新主机 C。

### 8.2 如果你用的是 DNS 切换

把 `DOMAIN` 及其相关子域名的 A/AAAA 记录改到新主机 C 的公网 IP。

### 8.3 建议保留一段“旧主机 A 反代到 C”的过渡期

如果你担心 DNS TTL 未完全生效，可以暂时保留 A 上的 Caddy，只做“全量转发到 C”，不要再让 A 跑旧 `sub2api`。

示例：

```caddy
# 将 DOMAIN 替换成你的真实域名
DOMAIN {
    reverse_proxy http://C_NEW_IP:80 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        header_up X-Forwarded-Host {host}
        header_up CF-Connecting-IP {http.request.header.CF-Connecting-IP}
    }
}
```

这里故意反代到 `C_NEW_IP:80`，而不是直连 `2127`，原因是：

- 这样旧 A 上命中的 HTTPS 请求会继续走新主机 C 的 Caddy
- 新主机 C 可以自己处理证书、HTTP->HTTPS 跳转、以及 ACME challenge
- 对迁移中的 HTTPS 修复最稳

这只是 TTL / ACME 过渡手段，不建议长期保留。

## 9. 新主服务器 C 上的 Caddy 建议

迁移后，如果 B 继续保留为应用节点，C 上的 Caddy 仍建议沿用扩容手册思路：

- 普通请求：`C 本机 + B` 轮询
- `/api/v1/admin/data-management/*`：固定到 C 本机

示例：

```caddy
# 将 example.com / www.example.com / api.example.com
# 替换成你的真实域名
example.com, www.example.com, api.example.com {
    }

    @data_management {
        path /api/v1/admin/data-management/*
    }

        reverse_proxy 127.0.0.1:2127
    }

    handle @data_management {
        reverse_proxy 127.0.0.1:2127
    }

    handle {
        reverse_proxy 127.0.0.1:2127 B_APP_IP:2127 {
            health_uri /health
            health_interval 30s
            health_timeout 10s
            health_status 200
            lb_policy round_robin
            lb_try_duration 5s
            lb_try_interval 250ms
        }
    }
}
```

如果迁移后不保留 B，把 `B_APP_IP:2127` 删除即可。

应用后检查：

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
sudo systemctl status caddy --no-pager
```

## 10. 迁移完成后的验收

至少检查以下内容：

### 10.1 服务状态

- C 上 `PostgreSQL` 正常
- C 上 `Redis` 正常
- C 上 `sub2api` 正常
- C 上 `Caddy` 正常
- B 上 `sub2api` 正常（如保留）

### 10.2 健康检查

```bash
curl http://127.0.0.1:2127/health
curl https://DOMAIN/health
curl https://api.DOMAIN/health
```

如果保留 B，再额外检查：

```bash
curl http://B_APP_IP:2127/health
```

### 10.3 运行时连通性

- B 的应用日志中能看到正常业务请求
- C 的 Redis 中能看到来自 B 的连接
- C 的 PostgreSQL 连接情况正常

可检查：

```bash
sudo journalctl -u sub2api -n 80 --no-pager
redis-cli -a 'REDIS_PASSWORD' CLIENT LIST
```

### 10.4 业务验证

至少验证：

- 登录态正常
- `/api/v1/settings/public` 正常
- `/v1/responses` 正常
- 数据管理功能正常（如启用）

## 11. 回滚方案

### 11.1 最稳的回滚窗口

最适合回滚的窗口是：

- 新主机 C 已启动
- 流量刚切过去不久
- 尚未产生必须保留的新写入

### 11.2 快速回滚步骤

1. 把 DNS / 浮动 IP 切回旧主机 A
2. 恢复 A 上原来的 Caddy 配置
3. 在 A 上重新启动：
   - `postgresql`
   - `redis`
   - `sub2api`
   - `sub2api-datamanagementd`（如启用）
4. 如果保留 B，把 B 的数据库/Redis 地址改回 A，并重启 B 的 `sub2api`

### 11.3 一个关键限制

如果 C 已经承接了一段时间生产写流量，不要直接把入口切回 A 就结束。

这时 A 上的数据已经落后于 C。若直接回切，会丢失切换后产生的新数据。

正确做法是至少先完成下面之一：

- 把 C 上新增的数据再同步回 A
- 接受“切换后新增数据丢失”的后果

### 11.4 迁移完成前不要删除 A 上的数据

在你确认 C 已稳定运行前，不要删除 A 上的：

- PostgreSQL 数据目录
- Redis 数据目录
- `datamanagementd` SQLite
- `/opt/sub2api`
- `/etc/caddy/Caddyfile`

## 12. 一句话建议

对于“把旧主服务器整体迁移到新高性能服务器”的场景，最稳妥的做法不是直接换机器，而是：

- 先把 C 预热成一个可运行节点
- 先预同步大数据
- 再用一次短停机窗口做最终切换
- 最后才让域名指向 C

这样改动最少、风险最低、回滚也最清晰。
