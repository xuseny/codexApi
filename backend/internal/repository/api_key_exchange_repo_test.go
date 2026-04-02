package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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
