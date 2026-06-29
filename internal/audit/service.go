package audit

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

type AuditService struct {
	db    *pgx.Conn
	queue chan AuditRequest
}

func NewAuditService(db *pgx.Conn) *AuditService {
	s := &AuditService{
		db:    db,
		queue: make(chan AuditRequest, 100),
	}
	go s.worker()
	return s
}

func (s *AuditService) worker() {
	for req := range s.queue {
		if err := s.create(req); err != nil {
			log.Printf("audit: failed to write record: %v", err)
		}
	}
}

func (s *AuditService) Enqueue(req AuditRequest) {
	s.queue <- req
}

func (s *AuditService) GetAll() ([]AuditRequest, error) {
	rows, err := s.db.Query(context.Background(), `SELECT actor_id, actor_type, event_type, status, ip_address, user_agent, created_at FROM audit_trail`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var audits []AuditRequest
	for rows.Next() {
		var a AuditRequest
		if err := rows.Scan(&a.ActorId, &a.ActorType, &a.EventType, &a.Status, &a.IPAddress, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, err
		}
		audits = append(audits, a)
	}
	return audits, rows.Err()
}

func (s *AuditService) create(req AuditRequest) error {
	_, err := s.db.Exec(context.Background(),
		`INSERT INTO audit_trail (actor_id, actor_type, event_type, status, ip_address, user_agent, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		req.ActorId, req.ActorType, req.EventType, req.Status, req.IPAddress, req.UserAgent, req.CreatedAt,
	)
	return err
}
