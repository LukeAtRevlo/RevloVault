package vault

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrValueNotFound = errors.New("value not found for this org and request instance")

type ValueService struct {
	db *pgxpool.Pool
}

func NewValueService(db *pgxpool.Pool) *ValueService {
	return &ValueService{db: db}
}

func generateValueKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "vault:v1:val:" + hex.EncodeToString(buf), nil
}

// StoreValue writes value into vault_values. If existingKey is empty, a
// fresh key is generated and a new row is inserted. If existingKey is set,
// it must already belong to this org and request instance; the row is
// updated in place via a single atomic UPDATE and the same key is returned.
// A non-matching existingKey is a hard error, never a silent fallback to
// inserting a new row.
func (s *ValueService) StoreValue(ctx context.Context, orgId string, requestInstanceId string, value string, existingKey string) (string, error) {
	if existingKey != "" {
		tag, err := s.db.Exec(ctx,
			`UPDATE vault_values SET value = $1, updated_at = now()
			 WHERE key = $2 AND org_id = $3::uuid AND request_instance_id = $4::uuid`,
			value, existingKey, orgId, requestInstanceId,
		)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 0 {
			return "", ErrValueNotFound
		}
		return existingKey, nil
	}

	key, err := generateValueKey()
	if err != nil {
		return "", err
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO vault_values (key, org_id, request_instance_id, value)
		 VALUES ($1, $2::uuid, $3::uuid, $4)`,
		key, orgId, requestInstanceId, value,
	)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (s *ValueService) GetValue(ctx context.Context, key string, orgId string, requestInstanceId string) (string, error) {
	var value string
	err := s.db.QueryRow(ctx,
		`SELECT value FROM vault_values WHERE key = $1 AND org_id = $2::uuid AND request_instance_id = $3::uuid`,
		key, orgId, requestInstanceId,
	).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrValueNotFound
		}
		return "", err
	}
	return value, nil
}
