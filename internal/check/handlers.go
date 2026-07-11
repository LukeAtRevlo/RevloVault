package check

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/LukeAtRevlo/RevloVault/internal/vault"
)

type CheckHandler struct {
	Service       *CheckService
	AMLService    *AMLService
	VaultService  *vault.VaultService
	WebhookSecret string
}

type AMLHTTPRequest struct {
	Name        string `json:"name"`
	BirthDate   string `json:"birthDate,omitempty"`
	Nationality string `json:"nationality,omitempty"`
	Country     string `json:"country,omitempty"`
}

func (h *CheckHandler) HandleAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AMLHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	result, err := h.AMLService.Check(AMLQuery{
		Name:        req.Name,
		BirthDate:   req.BirthDate,
		Nationality: req.Nationality,
		Country:     req.Country,
	})
	if err != nil {
		log.Printf("AML check error: %v", err)
		http.Error(w, "Service error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type IdentityDocument struct {
	Key       string `json:"key"`
	IDDocType string `json:"idDocType"`
	Country   string `json:"country"`
}

type IdentityRequest struct {
	ExternalUserID string             `json:"externalUserId"`
	LevelName      string             `json:"levelName"`
	Email          string             `json:"email,omitempty"`
	Phone          string             `json:"phone,omitempty"`
	FixedInfo      FixedInfo          `json:"fixedInfo,omitempty"`
	Documents      []IdentityDocument `json:"documents"`
}

func (h *CheckHandler) HandleIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}


	var req IdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ExternalUserID == "" || req.LevelName == "" || len(req.Documents) == 0 {
		http.Error(w, "externalUserId, levelName and at least one document are required", http.StatusBadRequest)
		return
	}

	applicant, err := h.Service.CreateApplicant(req.LevelName, CreateApplicantRequest{
		ExternalUserID: req.ExternalUserID,
		Type:           "individual",
		Email:          req.Email,
		Phone:          req.Phone,
		FixedInfo:      req.FixedInfo,
	})
	if err != nil {
		log.Printf("CreateApplicant error: %v", err)
		http.Error(w, "Failed to create applicant", http.StatusInternalServerError)
		return
	}

	for _, doc := range req.Documents {
		fileContent, err := h.VaultService.ReadFile(r.Context(), doc.Key)
		if err != nil {
			log.Printf("ReadFile error for key %s: %v", doc.Key, err)
			http.Error(w, "Failed to read document: "+doc.Key, http.StatusInternalServerError)
			return
		}

		fileName, err := h.VaultService.FileName(doc.Key)
		if err != nil {
			log.Printf("FileName error for key %s: %v", doc.Key, err)
			http.Error(w, "Failed to read document: "+doc.Key, http.StatusInternalServerError)
			return
		}

		if err := h.Service.SubmitDocument(applicant.ID, DocumentMetadata{
			IDDocType: doc.IDDocType,
			Country:   doc.Country,
		}, fileContent, fileName); err != nil {
			log.Printf("SubmitDocument error for key %s: %v", doc.Key, err)
			http.Error(w, "Failed to upload document: "+doc.Key, http.StatusInternalServerError)
			return
		}
	}

	if err := h.Service.SubmitForReview(applicant.ID); err != nil {
		log.Printf("SubmitForReview error: %v", err)
		http.Error(w, "Failed to submit for review", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"applicantId": applicant.ID,
		"status":      "pending",
	})
}

func (h *CheckHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Webhook read error: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	if !h.verifyWebhookSignature(r, body) {
		log.Printf("Webhook signature verification failed")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	log.Printf("Sumsub webhook received: %s", body)

	w.WriteHeader(http.StatusOK)
}

func (h *CheckHandler) verifyWebhookSignature(r *http.Request, body []byte) bool {
	if h.WebhookSecret == "" {
		log.Printf("Webhook secret not configured")
		return false
	}

	if r.Header.Get("X-Payload-Digest-Alg") != "HMAC_SHA256" {
		return false
	}

	sig := r.Header.Get("X-Payload-Digest")
	if sig == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.WebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(sig))
}
