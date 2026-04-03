package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyExchangeRepositoryDelete_ActivatedCodeAllowed(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &apiKeyExchangeRepository{db: db}

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM api_key_exchange_codes WHERE id = $1")).
		WithArgs(int64(12)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(context.Background(), 12)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyExchangeRepositoryDelete_DisabledCodeAllowed(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &apiKeyExchangeRepository{db: db}

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM api_key_exchange_codes WHERE id = $1")).
		WithArgs(int64(18)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(context.Background(), 18)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyExchangeRepositoryRedeemQuota_AcceptsUnusedExchangeCodeAsRechargeSource(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &apiKeyExchangeRepository{db: db}

	now := time.Now().UTC()

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			id,
			code,
			owner_user_id,
			created_by,
			group_id,
			quota,
			expires_in_days,
			status,
			api_key_id,
			activated_at,
			activated_ip,
			batch_no,
			notes,
			created_at,
			updated_at
		FROM api_key_exchange_codes
		WHERE code = $1
		FOR UPDATE
	`)).
		WithArgs("BC89-1A8F-48DC-0347").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "code", "owner_user_id", "created_by", "group_id", "quota", "expires_in_days", "status", "api_key_id", "activated_at", "activated_ip", "batch_no", "notes", "created_at", "updated_at",
		}).AddRow(
			int64(21), "BC89-1A8F-48DC-0347", int64(1), int64(1), int64(7), 10.0, 30, service.APIKeyExchangeStatusActivated, int64(99), now, nil, "batch-a", "", now, now,
		))

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, user_id, key, name, group_id, status, quota, quota_used, expires_at, last_used_at
		FROM api_keys
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`)).
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "key", "name", "group_id", "status", "quota", "quota_used", "expires_at", "last_used_at",
		}).AddRow(
			int64(99), int64(1), "sk-target", "target-key", int64(7), service.StatusAPIKeyQuotaExhausted, 10.0, 10.0, nil, nil,
		))

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, code, type, value, status
		FROM redeem_codes
		WHERE code = $1
		FOR UPDATE
	`)).
		WithArgs("715F-536A-A173-73CD").
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "type", "value", "status"}))

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			id,
			code,
			owner_user_id,
			created_by,
			group_id,
			quota,
			expires_in_days,
			status,
			api_key_id,
			activated_at,
			activated_ip,
			batch_no,
			notes,
			created_at,
			updated_at
		FROM api_key_exchange_codes
		WHERE code = $1
		FOR UPDATE
	`)).
		WithArgs("715F-536A-A173-73CD").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "code", "owner_user_id", "created_by", "group_id", "quota", "expires_in_days", "status", "api_key_id", "activated_at", "activated_ip", "batch_no", "notes", "created_at", "updated_at",
		}).AddRow(
			int64(22), "715F-536A-A173-73CD", int64(1), int64(1), int64(7), 5.0, 0, service.APIKeyExchangeStatusUnused, nil, nil, nil, "batch-b", "", now, now,
		))

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE api_key_exchange_codes
		 SET status = $1, api_key_id = $2, group_id = $3, activated_at = $4, updated_at = $4
		 WHERE id = $5 AND status = $6 AND api_key_id IS NULL
	`)).
		WithArgs(service.APIKeyExchangeStatusActivated, int64(99), sql.NullInt64{Int64: 7, Valid: true}, sqlmock.AnyArg(), int64(22), service.APIKeyExchangeStatusUnused).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(regexp.QuoteMeta(`
		UPDATE api_keys
		 SET
			quota = quota + $1,
			status = CASE
				WHEN status = $2 AND quota + $1 > quota_used THEN $3
				ELSE status
			END,
			updated_at = $4
		 WHERE id = $5 AND deleted_at IS NULL
		 RETURNING id
	`)).
		WithArgs(5.0, service.StatusAPIKeyQuotaExhausted, service.StatusAPIKeyActive, sqlmock.AnyArg(), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			c.id,
			c.code,
			c.owner_user_id,
			c.created_by,
			c.group_id,
			c.quota,
			c.expires_in_days,
			c.status,
			c.api_key_id,
			c.activated_at,
			c.activated_ip,
			c.batch_no,
			c.notes,
			c.created_at,
			c.updated_at,
			g.id,
			g.name,
			g.platform,
			g.status,
			g.subscription_type,
			g.rate_multiplier,
			k.id,
			k.key,
			k.name,
			k.status,
			k.quota,
			k.quota_used,
			k.expires_at,
			k.last_used_at
		FROM api_key_exchange_codes c
		LEFT JOIN groups g ON g.id = c.group_id
		LEFT JOIN api_keys k ON k.id = c.api_key_id AND k.deleted_at IS NULL
		WHERE c.id = $1
	`)).
		WithArgs(int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "code", "owner_user_id", "created_by", "group_id", "quota", "expires_in_days", "status", "api_key_id", "activated_at", "activated_ip", "batch_no", "notes", "created_at", "updated_at",
			"group_entity_id", "group_name", "group_platform", "group_status", "group_subscription_type", "group_rate_multiplier",
			"api_key_entity_id", "api_key_value", "api_key_name", "api_key_status", "api_key_quota", "api_key_quota_used", "api_key_expires_at", "api_key_last_used_at",
		}).AddRow(
			int64(21), "BC89-1A8F-48DC-0347", int64(1), int64(1), int64(7), 10.0, 30, service.APIKeyExchangeStatusActivated, int64(99), now, nil, "batch-a", "", now, now,
			int64(7), "OpenAI", service.PlatformOpenAI, service.StatusActive, service.SubscriptionTypeStandard, 1.0,
			int64(99), "sk-target", "target-key", service.StatusAPIKeyActive, 15.0, 10.0, nil, nil,
		))

	mock.ExpectCommit()

	item, amount, err := repo.RedeemQuota(context.Background(), "BC89-1A8F-48DC-0347", "715F-536A-A173-73CD")
	require.NoError(t, err)
	require.Equal(t, 5.0, amount)
	require.NotNil(t, item)
	require.NotNil(t, item.APIKey)
	require.Equal(t, 15.0, item.APIKey.Quota)
	require.Equal(t, service.StatusAPIKeyActive, item.APIKey.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}
