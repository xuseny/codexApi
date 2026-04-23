# Sub2API 服务器磁盘清理脚本

这个目录新增了一个可复用的 Linux 清理脚本：

- `/root/deploy/server_disk_cleanup.sh`

它是为你当前这台机器上已经确认存在的占用来源准备的，默认只做预览，不会直接删文件。

## 默认会处理什么

- 删除 `/www/server/pgsql/logs` 里较旧的 PostgreSQL 日志
- 清理 APT 缓存
- 删除轮转后的 `syslog` 文件
- 在 `syslog` 超过阈值时清空当前 `syslog`
- 把 `systemd journal` 压到指定大小
- 清空过大的 Docker `*-json.log`
- 清理 Go build cache 和模块下载缓存

## 明确不会碰什么

- `/swap.img`
- `/www/server/pgsql/data`
- `/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots`
- `/root/*.dump`
- `/root/sub2api.from-A`
- `/opt/sub2api/sub2api`

也就是说，这个脚本不会直接删 PostgreSQL 数据文件、swap、容器层、备份 dump 和正在用的主程序二进制。

## 最常用的用法

先预览：

```bash
bash /root/deploy/server_disk_cleanup.sh
```

确认无误后执行：

```bash
sudo bash /root/deploy/server_disk_cleanup.sh --apply
```

如果你下次想直接跑，不想再确认一次：

```bash
sudo bash /root/deploy/server_disk_cleanup.sh --apply --yes
```

## 可调参数

默认配置比较保守：

- 保留最新 `2` 个 PostgreSQL 日志文件
- `journal` 目标大小 `200M`
- Docker 日志超过 `100M` 才清空
- 当前 `syslog` 超过 `100M` 才清空

你可以这样改：

```bash
sudo bash deploy/server_disk_cleanup.sh --apply --keep-pg-logs 3
sudo bash deploy/server_disk_cleanup.sh --apply --journal-size 300M
sudo bash deploy/server_disk_cleanup.sh --apply --docker-log-threshold 200M
sudo bash deploy/server_disk_cleanup.sh --apply --syslog-threshold 150M
```

也支持用环境变量覆盖：

```bash
sudo PGSQL_LOG_KEEP_COUNT=3 JOURNAL_MAX_SIZE=300M bash deploy/server_disk_cleanup.sh --apply --yes
```

## 建议执行顺序

第一次先跑：

```bash
bash deploy/server_disk_cleanup.sh
```

看清楚脚本准备删哪些文件以后，再执行：

```bash
sudo bash deploy/server_disk_cleanup.sh --apply
```

## 清理后还没释放空间怎么办

如果文件已经删了，磁盘空间还是没回来，通常是进程还占着已经删除的文件句柄。检查：

```bash
sudo lsof +L1
```

看到对应进程后，再决定是否重启服务。

## 适合做成定时任务

如果你确认这套策略就是你想要的，可以加一个 cron，比如每天凌晨 4 点执行一次：

```bash
0 4 * * * /bin/bash /opt/sub2api/deploy/server_disk_cleanup.sh --apply --yes >> /var/log/sub2api-cleanup.log 2>&1
```

前提是你把仓库或脚本放在稳定路径，并确认路径和权限都正确。
