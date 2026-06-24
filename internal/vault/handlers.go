package vault

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type GrantUploadRequest struct {
	FileName    string `json:"fileName"`
	OrgID       string `json:"orgId"`
	ClientID    string `json:"clientId"`
	ContentType string `json:"contentType"`
}

type VaultHandler struct {
	Service *VaultService
}

func (h *VaultHandler) HandleGrantUpload(w http.ResponseWriter, r *http.Request) {
	var req GrantUploadRequest
	println("Upload")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	url, key, err := h.Service.GetUploadURL(r.Context(), req.FileName, req.OrgID, req.ClientID, req.ContentType)
	log.Printf("GetUploadURL result: url=%s key=%s err=%v", url, key, err)
	if err != nil {
		log.Printf("GetUploadURL error: %v", err)
		http.Error(w, "Service Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
		"key": key,
	})
}

type GrantDownloadRequest struct {
	Key   string `json:"key"`
	OrgID string `json:"orgId"`
}

func (h *VaultHandler) HandleGrantDownload(w http.ResponseWriter, r *http.Request) {
	var req GrantDownloadRequest

        log.Printf("Download")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Key == "" || req.OrgID == "" {
		http.Error(w, "key and orgId are required", http.StatusBadRequest)
		return
	}

	expectedPrefix := "vault/" + req.OrgID + "/"
	if !strings.HasPrefix(req.Key, expectedPrefix) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	url, err := h.Service.GetDownloadURL(r.Context(), req.Key)
	if err != nil {
		http.Error(w, "Service Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})
}