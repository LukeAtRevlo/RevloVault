package audit

import (
	"encoding/json"
	"net/http"
)

type AuditHandler struct {
	Service *AuditService
}

func NewAuditHandler(service *AuditService) *AuditHandler {
	return &AuditHandler{Service: service}
}

type AuditRequest struct {
	ActorId   string `json:"actor_id"`
	ActorType string `json:"actor_type"`
	EventType string `json:"event_type"`
	Status    string `json:"status"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	CreatedAt string `json:"created_at"`
	AuditType string 
}


func (h *AuditHandler) HandleCreateAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.Service.Enqueue(req)

	w.WriteHeader(http.StatusAccepted)
}

func (h *AuditHandler) HandleGetAudits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audits, err := h.Service.GetAll()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(audits)
}
