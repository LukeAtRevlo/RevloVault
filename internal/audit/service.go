package audit

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type AuditService struct {
	db *sql.DB
}

func NewAuditService(dsn string) (*AuditService, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to audit DB: %w", err)
	}
	return &AuditService{db: db}, nil
}

type AuditEvent struct {
	TransactionReference string         `json:"transactionReference"`
	ActorID              string         `json:"actorId"`
	ActorType            string         `json:"actorType"`
	OrgID                string         `json:"orgId"`
	IPAddress            string         `json:"ipAddress"`
	UserAgent            string         `json:"userAgent"`
	EventCategory        string         `json:"eventCategory"`
	EventAction          string         `json:"eventAction"`
	TargetClientID       string         `json:"targetClientId"`
	TargetResourceID     string         `json:"targetResourceId,omitempty"`
	EventContext         map[string]any `json:"eventContext"`
	Justification        string         `json:"justification,omitempty"`
}

var validActorTypes = map[string]bool{
	"Client":        true,
	"External_User": true,
	"Internal_User": true,
	"System": 		 true,
}

func (s *AuditService) LogEvent(event AuditEvent) error {
	if !validActorTypes[event.ActorType] {
		return fmt.Errorf("invalid actor_type: %s", event.ActorType)
	}

	contextJSON, err := json.Marshal(event.EventContext)
	if err != nil {
		return fmt.Errorf("failed to marshal event_context: %w", err)
	}

	hash := sha256.Sum256(contextJSON)
	payloadHash := fmt.Sprintf("%x", hash)

	_, err = s.db.Exec(`
		INSERT INTO event_audit_ledger (
			transaction_reference, actor_id, actor_type, org_id,
			ip_address, user_agent, event_category, event_action,
			target_client_id, target_resource_id, event_context,
			payload_hash, justification
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.TransactionReference,
		event.ActorID,
		event.ActorType,
		event.OrgID,
		event.IPAddress,
		event.UserAgent,
		event.EventCategory,
		event.EventAction,
		event.TargetClientID,
		nullableString(event.TargetResourceID),
		string(contextJSON),
		payloadHash,
		nullableString(event.Justification),
	)
	return err
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
