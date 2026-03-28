package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type apiKeyExchangeRepository struct {
	db *sql.DB
}

func NewAPIKeyExchangeRepository(sqlDB *sql.DB) service.APIKeyExchangeRepository {
	return &apiKeyExchangeRepository{db: sqlDB}
}

func (r *apiKeyExchangeRepository) CreateBatch(ctx context.Context, codes []service.APIKeyExchangeCode) error {
	if len(codes) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		INSERT INTO api_key_exchange_codes
			(code, owner_user_id, created_by, group_id, quota, expires_in_days, status, batch_no, notes, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	for i := range codes {
		var createdBy sql.NullInt64
		if codes[i].CreatedBy != nil {
			createdBy.Valid = true
			createdBy.Int64 = *codes[i].CreatedBy
		}
		var groupID sql.NullInt64
		if codes[i].GroupID != nil {
			groupID.Valid = true
			groupID.Int64 = *codes[i].GroupID
		}

		if err := tx.QueryRowContext(
			ctx,
			query,
			codes[i].Code,
			codes[i].OwnerUserID,
			createdBy,
			groupID,
			codes[i].Quota,
			codes[i].ExpiresInDays,
			codes[i].Status,
			codes[i].BatchNo,
			codes[i].Notes,
		).Scan(&codes[i].ID, &codes[i].CreatedAt, &codes[i].UpdatedAt); err != nil {
			if isUniqueConstraintViolation(err) {
				return service.ErrAPIKeyExists.WithCause(err)
			}
			return err
		}
	}

	return tx.Commit()
}

func (r *apiKeyExchangeRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.APIKeyExchangeCodeListFilters) ([]service.APIKeyExchangeCode, *pagination.PaginationResult, error) {
	where, args := r.buildListWhere(filters)

	countQuery := "SELECT COUNT(*) FROM api_key_exchange_codes c LEFT JOIN api_keys k ON k.id = c.api_key_id AND k.deleted_at IS NULL" + where
	var total int64
	if err := scanSingleRow(ctx, r.db, countQuery, args, &total); err != nil {
		return nil, nil, err
	}

	query := `
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
	` + where + `
		ORDER BY c.id DESC
		LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.APIKeyExchangeCode, 0)
	for rows.Next() {
		item, err := scanAPIKeyExchangeCode(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return items, paginationResultFromTotal(total, params), nil
}

func (r *apiKeyExchangeRepository) GetByID(ctx context.Context, id int64) (*service.APIKeyExchangeCode, error) {
	query := `
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
	`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, service.ErrAPIKeyExchangeCodeNotFound
	}

	item, err := scanAPIKeyExchangeCode(rows)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *apiKeyExchangeRepository) DeleteUnused(ctx context.Context, id int64) error {
	var status string
	if err := scanSingleRow(ctx, r.db, "SELECT status FROM api_key_exchange_codes WHERE id = $1", []any{id}, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrAPIKeyExchangeCodeNotFound
		}
		return err
	}
	if status != service.APIKeyExchangeStatusUnused {
		return service.ErrAPIKeyExchangeDeleteActivated
	}

	res, err := r.db.ExecContext(ctx, "DELETE FROM api_key_exchange_codes WHERE id = $1", id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyExchangeCodeNotFound
	}
	return nil
}

func (r *apiKeyExchangeRepository) Resolve(ctx context.Context, code string, apiKeyName string, apiKeyValue string, activatedIP string) (*service.APIKeyExchangeCode, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()

	record, err := r.getByCodeForUpdate(ctx, tx, code)
	if err != nil {
		return nil, "", err
	}
	if record.Status == service.APIKeyExchangeStatusDisabled {
		return nil, "", service.ErrAPIKeyExchangeCodeDisabled
	}

	action := service.APIKeyExchangeActionQueried
	if record.Status == service.APIKeyExchangeStatusUnused {
		action = service.APIKeyExchangeActionActivated
		apiKeyID, apiKeyCreatedAt, apiKeyUpdatedAt, err := r.insertAPIKeyFromExchange(ctx, tx, record, apiKeyName, apiKeyValue)
		if err != nil {
			return nil, "", err
		}

		now := time.Now()
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE api_key_exchange_codes
			 SET status = $1, api_key_id = $2, activated_at = $3, activated_ip = $4, updated_at = $3
			 WHERE id = $5`,
			service.APIKeyExchangeStatusActivated,
			apiKeyID,
			now,
			nullStringValue(activatedIP),
			record.ID,
		); err != nil {
			return nil, "", err
		}

		record.Status = service.APIKeyExchangeStatusActivated
		record.APIKeyID = &apiKeyID
		record.ActivatedAt = &now
		record.ActivatedIP = nullableStringPtr(activatedIP)
		record.UpdatedAt = now
		record.APIKey = &service.APIKey{
			ID:        apiKeyID,
			Key:       apiKeyValue,
			Name:      apiKeyName,
			UserID:    record.OwnerUserID,
			GroupID:   record.GroupID,
			Status:    service.StatusAPIKeyActive,
			Quota:     record.Quota,
			QuotaUsed: 0,
			CreatedAt: apiKeyCreatedAt,
			UpdatedAt: apiKeyUpdatedAt,
		}
		if record.ExpiresInDays > 0 {
			expiresAt := now.AddDate(0, 0, record.ExpiresInDays)
			record.APIKey.ExpiresAt = &expiresAt
		}
	}

	item, err := r.getByIDWithExecutor(ctx, tx, record.ID)
	if err != nil {
		return nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	return item, action, nil
}

func (r *apiKeyExchangeRepository) GetUsageSummary(ctx context.Context, apiKeyID int64, todayStart, end time.Time) (*service.APIKeyExchangeUsageSummary, error) {
	query := `
		SELECT
			(SELECT COUNT(*) FROM usage_logs WHERE api_key_id = $1) AS total_requests,
			(SELECT COALESCE(SUM(actual_cost), 0) FROM usage_logs WHERE api_key_id = $1 AND created_at >= $2 AND created_at < $3) AS today_actual_cost,
			(SELECT COALESCE(SUM(actual_cost), 0) FROM usage_logs WHERE api_key_id = $1) AS total_actual_cost
	`

	var summary service.APIKeyExchangeUsageSummary
	if err := scanSingleRow(ctx, r.db, query, []any{apiKeyID, todayStart, end}, &summary.TotalRequests, &summary.TodayActualCost, &summary.TotalActualCost); err != nil {
		return nil, err
	}
	return &summary, nil
}

func (r *apiKeyExchangeRepository) buildListWhere(filters service.APIKeyExchangeCodeListFilters) (string, []any) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if filters.Status != "" {
		clauses = append(clauses, "c.status = $"+itoa(len(args)+1))
		args = append(args, filters.Status)
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		clauses = append(clauses, "(c.code ILIKE $"+itoa(len(args)+1)+" OR c.batch_no ILIKE $"+itoa(len(args)+1)+" OR COALESCE(k.key, '') ILIKE $"+itoa(len(args)+1)+")")
		args = append(args, "%"+search+"%")
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (r *apiKeyExchangeRepository) getByCodeForUpdate(ctx context.Context, tx *sql.Tx, code string) (*service.APIKeyExchangeCode, error) {
	query := `
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
	`

	rows, err := tx.QueryContext(ctx, query, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, service.ErrAPIKeyExchangeCodeNotFound
	}

	var (
		item         service.APIKeyExchangeCode
		createdBy    sql.NullInt64
		groupID      sql.NullInt64
		apiKeyID     sql.NullInt64
		activatedAt  sql.NullTime
		activatedIP  sql.NullString
		batchNo      sql.NullString
		notes        sql.NullString
	)
	if err := rows.Scan(
		&item.ID,
		&item.Code,
		&item.OwnerUserID,
		&createdBy,
		&groupID,
		&item.Quota,
		&item.ExpiresInDays,
		&item.Status,
		&apiKeyID,
		&activatedAt,
		&activatedIP,
		&batchNo,
		&notes,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		v := createdBy.Int64
		item.CreatedBy = &v
	}
	if groupID.Valid {
		v := groupID.Int64
		item.GroupID = &v
	}
	if apiKeyID.Valid {
		v := apiKeyID.Int64
		item.APIKeyID = &v
	}
	if activatedAt.Valid {
		v := activatedAt.Time
		item.ActivatedAt = &v
	}
	if activatedIP.Valid {
		v := activatedIP.String
		item.ActivatedIP = &v
	}
	if batchNo.Valid {
		item.BatchNo = batchNo.String
	}
	if notes.Valid {
		item.Notes = notes.String
	}
	return &item, nil
}

func (r *apiKeyExchangeRepository) insertAPIKeyFromExchange(ctx context.Context, tx *sql.Tx, record *service.APIKeyExchangeCode, apiKeyName string, apiKeyValue string) (int64, time.Time, time.Time, error) {
	query := `
		INSERT INTO api_keys
			(user_id, key, name, group_id, status, quota, quota_used, expires_at, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, 0, $7, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	var (
		apiKeyID   int64
		createdAt  time.Time
		updatedAt  time.Time
		expiresAt  sql.NullTime
		groupIDArg any
	)
	if record.GroupID != nil {
		groupIDArg = *record.GroupID
	}
	if record.ExpiresInDays > 0 {
		expiresAt.Valid = true
		expiresAt.Time = time.Now().AddDate(0, 0, record.ExpiresInDays)
	}

	if err := tx.QueryRowContext(
		ctx,
		query,
		record.OwnerUserID,
		apiKeyValue,
		apiKeyName,
		groupIDArg,
		service.StatusAPIKeyActive,
		record.Quota,
		expiresAt,
	).Scan(&apiKeyID, &createdAt, &updatedAt); err != nil {
		if isUniqueConstraintViolation(err) {
			return 0, time.Time{}, time.Time{}, service.ErrAPIKeyExists.WithCause(err)
		}
		return 0, time.Time{}, time.Time{}, err
	}

	return apiKeyID, createdAt, updatedAt, nil
}

func (r *apiKeyExchangeRepository) getByIDWithExecutor(ctx context.Context, exec sqlExecutor, id int64) (*service.APIKeyExchangeCode, error) {
	query := `
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
	`

	rows, err := exec.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, service.ErrAPIKeyExchangeCodeNotFound
	}

	item, err := scanAPIKeyExchangeCode(rows)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAPIKeyExchangeCode(scanner interface{ Scan(dest ...any) error }) (service.APIKeyExchangeCode, error) {
	var (
		item              service.APIKeyExchangeCode
		createdBy         sql.NullInt64
		groupID           sql.NullInt64
		apiKeyID          sql.NullInt64
		activatedAt       sql.NullTime
		activatedIP       sql.NullString
		batchNo           sql.NullString
		notes             sql.NullString
		groupEntityID     sql.NullInt64
		groupName         sql.NullString
		groupPlatform     sql.NullString
		groupStatus       sql.NullString
		groupSubType      sql.NullString
		groupRate         sql.NullFloat64
		apiKeyEntityID    sql.NullInt64
		apiKeyValue       sql.NullString
		apiKeyName        sql.NullString
		apiKeyStatus      sql.NullString
		apiKeyQuota       sql.NullFloat64
		apiKeyQuotaUsed   sql.NullFloat64
		apiKeyExpiresAt   sql.NullTime
		apiKeyLastUsedAt  sql.NullTime
	)

	err := scanner.Scan(
		&item.ID,
		&item.Code,
		&item.OwnerUserID,
		&createdBy,
		&groupID,
		&item.Quota,
		&item.ExpiresInDays,
		&item.Status,
		&apiKeyID,
		&activatedAt,
		&activatedIP,
		&batchNo,
		&notes,
		&item.CreatedAt,
		&item.UpdatedAt,
		&groupEntityID,
		&groupName,
		&groupPlatform,
		&groupStatus,
		&groupSubType,
		&groupRate,
		&apiKeyEntityID,
		&apiKeyValue,
		&apiKeyName,
		&apiKeyStatus,
		&apiKeyQuota,
		&apiKeyQuotaUsed,
		&apiKeyExpiresAt,
		&apiKeyLastUsedAt,
	)
	if err != nil {
		return service.APIKeyExchangeCode{}, err
	}

	if createdBy.Valid {
		v := createdBy.Int64
		item.CreatedBy = &v
	}
	if groupID.Valid {
		v := groupID.Int64
		item.GroupID = &v
	}
	if apiKeyID.Valid {
		v := apiKeyID.Int64
		item.APIKeyID = &v
	}
	if activatedAt.Valid {
		v := activatedAt.Time
		item.ActivatedAt = &v
	}
	if activatedIP.Valid {
		v := activatedIP.String
		item.ActivatedIP = &v
	}
	if batchNo.Valid {
		item.BatchNo = batchNo.String
	}
	if notes.Valid {
		item.Notes = notes.String
	}

	if groupEntityID.Valid {
		item.Group = &service.Group{
			ID:               groupEntityID.Int64,
			Name:             groupName.String,
			Platform:         groupPlatform.String,
			Status:           groupStatus.String,
			SubscriptionType: groupSubType.String,
			RateMultiplier:   groupRate.Float64,
		}
	}

	if apiKeyEntityID.Valid {
		item.APIKey = &service.APIKey{
			ID:        apiKeyEntityID.Int64,
			Key:       apiKeyValue.String,
			Name:      apiKeyName.String,
			Status:    apiKeyStatus.String,
			Quota:     apiKeyQuota.Float64,
			QuotaUsed: apiKeyQuotaUsed.Float64,
			GroupID:   item.GroupID,
		}
		if apiKeyExpiresAt.Valid {
			v := apiKeyExpiresAt.Time
			item.APIKey.ExpiresAt = &v
		}
		if apiKeyLastUsedAt.Valid {
			v := apiKeyLastUsedAt.Time
			item.APIKey.LastUsedAt = &v
		}
		item.APIKey.Group = item.Group
	}

	return item, nil
}

func nullableStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func nullStringValue(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
