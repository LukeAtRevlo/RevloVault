package check

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"

	"github.com/LukeAtRevlo/RevloVault/internal/vault"
)

type CheckHandler struct {
	Service      *CheckService
	VaultService *vault.VaultService
}

type CreateApplicantHTTPRequest struct {
	ExternalUserID string    `json:"externalUserId"`
	Type           string    `json:"type"`
	Email          string    `json:"email,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	FixedInfo      FixedInfo `json:"fixedInfo,omitempty"`
	LevelName      string    `json:"levelName"`
}

func (h *CheckHandler) HandleCreateApplicant(w http.ResponseWriter, r *http.Request) {
	var req CreateApplicantHTTPRequest
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ExternalUserID == "" || req.LevelName == "" {
		http.Error(w, "externalUserId and levelName are required", http.StatusBadRequest)
		return
	}

	result, err := h.Service.CreateApplicant(req.LevelName, CreateApplicantRequest{
		ExternalUserID: req.ExternalUserID,
		Type:           req.Type,
		Email:          req.Email,
		Phone:          req.Phone,
		FixedInfo:      req.FixedInfo,
	})
	if err != nil {
		log.Printf("CreateApplicant error: %v", err)
		http.Error(w, "Service Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type SubmitKYCDocument struct {
	Key       string `json:"key"`
	IDDocType string `json:"idDocType"`
	Country   string `json:"country"`
}

type SubmitKYCRequest struct {
	ExternalUserID string             `json:"externalUserId"`
	LevelName      string             `json:"levelName"`
	Email          string             `json:"email,omitempty"`
	Phone          string             `json:"phone,omitempty"`
	FixedInfo      FixedInfo          `json:"fixedInfo,omitempty"`
	Documents      []SubmitKYCDocument `json:"documents"`
}

func (h *CheckHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	var req SubmitKYCRequest
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

		if err := h.Service.SubmitDocument(applicant.ID, DocumentMetadata{
			IDDocType: doc.IDDocType,
			Country:   doc.Country,
		}, fileContent, filepath.Base(doc.Key)); err != nil {
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

type SubmitDocumentHTTPRequest struct {
	ApplicantID string `json:"applicantId"`
	Key         string `json:"key"`
	IDDocType   string `json:"idDocType"`
	Country     string `json:"country"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	Number      string `json:"number,omitempty"`
	DOB         string `json:"dob,omitempty"`
	ValidUntil  string `json:"validUntil,omitempty"`
}

func (h *CheckHandler) HandleSubmitDocument(w http.ResponseWriter, r *http.Request) {
	var req SubmitDocumentHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ApplicantID == "" || req.Key == "" || req.IDDocType == "" || req.Country == "" {
		http.Error(w, "applicantId, key, idDocType and country are required", http.StatusBadRequest)
		return
	}

	fileContent, err := h.VaultService.ReadFile(r.Context(), req.Key)
	if err != nil {
		log.Printf("ReadFile error: %v", err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	err = h.Service.SubmitDocument(req.ApplicantID, DocumentMetadata{
		IDDocType:  req.IDDocType,
		Country:    req.Country,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Number:     req.Number,
		DOB:        req.DOB,
		ValidUntil: req.ValidUntil,
	}, fileContent, filepath.Base(req.Key))
	
	if err != nil {
		log.Printf("SubmitDocument error: %v", err)
		http.Error(w, "Service Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type RecheckAMLHTTPRequest struct {
	ApplicantID string `json:"applicantId"`
}

func (h *CheckHandler) HandleRecheckAML(w http.ResponseWriter, r *http.Request) {
	var req RecheckAMLHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ApplicantID == "" {
		http.Error(w, "applicantId is required", http.StatusBadRequest)
		return
	}

	result, err := h.Service.RecheckAML(req.ApplicantID)
	if err != nil {
		log.Printf("RecheckAML error: %v", err)
		http.Error(w, "Service Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
