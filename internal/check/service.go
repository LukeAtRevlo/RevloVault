package check

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

const sumsubBaseURL = "https://api.sumsub.com"

type CheckService struct {
	apiKey    string
	apiSecret string
	client    *http.Client
}

func NewCheckService(apiKey, apiSecret string) *CheckService {
	return &CheckService{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

type Address struct {
	Country        string `json:"country,omitempty"`
	PostCode       string `json:"postCode,omitempty"`
	Town           string `json:"town,omitempty"`
	Street         string `json:"street,omitempty"`
	BuildingNumber string `json:"buildingNumber,omitempty"`
	FlatNumber     string `json:"flatNumber,omitempty"`
	State          string `json:"state,omitempty"`
}

type FixedInfo struct {
	FirstName   string    `json:"firstName,omitempty"`
	LastName    string    `json:"lastName,omitempty"`
	MiddleName  string    `json:"middleName,omitempty"`
	DOB         string    `json:"dob,omitempty"` 
	Gender      string    `json:"gender,omitempty"`
	Country     string    `json:"country,omitempty"` 
	Nationality string    `json:"nationality,omitempty"`
	Addresses   []Address `json:"addresses,omitempty"`
}

type CreateApplicantRequest struct {
	ExternalUserID string    `json:"externalUserId"`
	Type           string    `json:"type"` 
	Email          string    `json:"email,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	FixedInfo      FixedInfo `json:"fixedInfo,omitempty"`
}

type CreateApplicantResponse struct {
	ID             string `json:"id"`
	ExternalUserID string `json:"externalUserId"`
	CreatedAt      string `json:"createdAt"`
	Review         struct {
		ReviewStatus string `json:"reviewStatus"`
	} `json:"review"`
}

func (s *CheckService) CreateApplicant(levelName string, req CreateApplicantRequest) (*CreateApplicantResponse, error) {
	body, err := json.Marshal(req)

	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/resources/applicants?levelName=%s", levelName)
	httpReq, err := http.NewRequest("POST", sumsubBaseURL+path, bytes.NewReader(body))

	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.sign(httpReq, body)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("sumsub %d: %s", resp.StatusCode, respBody)
	}

	var result CreateApplicantResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type AMLCheckResponse struct {
	ApplicantID string `json:"applicantId"`
	Inspections []struct {
		InspectionID string `json:"inspectionId"`
		Status       string `json:"status"`
	} `json:"inspections"`
}

func (s *CheckService) RecheckAML(applicantID string) (*AMLCheckResponse, error) {
	path := fmt.Sprintf("/resources/applicants/%s/recheck/aml", applicantID)
	httpReq, err := http.NewRequest("POST", sumsubBaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	s.sign(httpReq, nil)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sumsub %d: %s", resp.StatusCode, respBody)
	}

	var result AMLCheckResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type DocumentMetadata struct {
	IDDocType  string `json:"idDocType"`
	Country    string `json:"country"`
	FirstName  string `json:"firstName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
	Number     string `json:"number,omitempty"`
	DOB        string `json:"dob,omitempty"`
	ValidUntil string `json:"validUntil,omitempty"`
}

func (s *CheckService) SubmitDocument(applicantID string, meta DocumentMetadata, fileContent []byte, fileName string) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	metaPart, _ := mw.CreateFormField("metadata")
	metaPart.Write(metaJSON)

	filePart, _ := mw.CreateFormFile("content", fileName)
	filePart.Write(fileContent)
	mw.Close()

	bodyBytes := buf.Bytes()
	path := fmt.Sprintf("/resources/applicants/%s/info/idDoc", applicantID)
	httpReq, err := http.NewRequest("POST", sumsubBaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	s.sign(httpReq, bodyBytes)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("sumsub %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

func (s *CheckService) SubmitForReview(applicantID string) error {
	path := fmt.Sprintf("/resources/applicants/%s/status/pending", applicantID)
	httpReq, err := http.NewRequest("POST", sumsubBaseURL+path, nil)
	if err != nil {
		return err
	}

	s.sign(httpReq, nil)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sumsub %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

func (s *CheckService) sign(req *http.Request, body []byte) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	msg := ts + req.Method + req.URL.RequestURI() + string(body)

	mac := hmac.New(sha256.New, []byte(s.apiSecret))
	mac.Write([]byte(msg))

	req.Header.Set("X-App-Token", s.apiKey)
	req.Header.Set("X-App-Access-Ts", ts)
	req.Header.Set("X-App-Access-Sig", hex.EncodeToString(mac.Sum(nil)))
}
