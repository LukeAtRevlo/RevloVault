package audit

import (
	"encoding/json"
	"log"
	"net/http"
)

type AuditHandler struct {
	Service *AuditService
}

func (h *AuditHandler) HandleLogEvent(w http.ResponseWriter, r *http.Request) {
	var event AuditEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if event.TransactionReference == "" || event.ActorID == "" || event.ActorType == "" ||
		event.OrgID == "" || event.EventCategory == "" || event.EventAction == "" ||
		event.TargetClientID == "" || event.EventContext == nil {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	if event.IPAddress == "" {
		event.IPAddress = r.RemoteAddr
	}
	if event.UserAgent == "" {
		event.UserAgent = r.UserAgent()
	}

	if err := h.Service.LogEvent(event); err != nil {
		log.Printf("LogEvent error: %v", err)
		http.Error(w, "Failed to log event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
