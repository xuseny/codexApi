-- Migration: 045_add_accounts_extra_index
-- 为 accounts.extra 字段添加 GIN 索引，优化 FindByExtraField 查询性能

CREATE INDEX IF NOT EXISTS idx_accounts_extra_gin
ON accounts USING GIN (extra);

-- 查询示例（使用 @> 操作符）
