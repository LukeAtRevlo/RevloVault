package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const openSanctionsBaseURL = "https://api.opensanctions.org"

type AMLService struct {
	apiKey string
	client *http.Client
}

func NewAMLService(apiKey string) *AMLService {
	return &AMLService{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type AMLQuery struct {
	Name        string `json:"name"`
	BirthDate   string `json:"birthDate,omitempty"`
	Nationality string `json:"nationality,omitempty"`
	Country     string `json:"country,omitempty"`
}

type AMLMatch struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Score      float64 `json:"score"`
	Match      bool    `json:"match"`
	Datasets   []string `json:"datasets"`
}

type AMLResult struct {
	Query   AMLQuery   `json:"query"`
	Matches []AMLMatch `json:"matches"`
	Total   int        `json:"total"`
}

func (s *AMLService) Check(query AMLQuery) (*AMLResult, error) {
	properties := map[string]any{
		"name": []string{query.Name},
	}
	if query.BirthDate != "" {
		properties["birthDate"] = []string{query.BirthDate}
	}
	if query.Nationality != "" {
		properties["nationality"] = []string{query.Nationality}
	}
	if query.Country != "" {
		properties["country"] = []string{query.Country}
	}

	payload := map[string]any{
		"queries": map[string]any{
			"subject": map[string]any{
				"schema":     "Person",
				"properties": properties,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", openSanctionsBaseURL+"/match/default", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "ApiKey "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensanctions %d: %s", resp.StatusCode, respBody)
	}

	var raw struct {
		Responses map[string]struct {
			Results []struct {
				ID       string   `json:"id"`
				Score    float64  `json:"score"`
				Match    bool     `json:"match"`
				Datasets []string `json:"datasets"`
				Properties struct {
					Name []string `json:"name"`
				} `json:"properties"`
			} `json:"results"`
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"responses"`
	}

	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, err
	}

	result := &AMLResult{Query: query}
	if subject, ok := raw.Responses["subject"]; ok {
		result.Total = subject.Total.Value
		for _, r := range subject.Results {
			name := ""
			if len(r.Properties.Name) > 0 {
				name = r.Properties.Name[0]
			}
			result.Matches = append(result.Matches, AMLMatch{
				ID:       r.ID,
				Name:     name,
				Score:    r.Score,
				Match:    r.Match,
				Datasets: r.Datasets,
			})
		}
	}

	return result, nil
}
