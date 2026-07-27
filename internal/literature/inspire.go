package literature

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
)

// InspireMeasurement represents a measurement from INSPIRE-HEP.
type InspireMeasurement struct {
	RecordID     string  `json:"record_id"`
	Collaboration string `json:"collaboration"`
	Observable   string  `json:"observable"`
	Value        float64 `json:"value"`
	Uncertainty  string  `json:"uncertainty"`
	Year         int     `json:"year"`
	DOI          string  `json:"doi"`
}

// InspireClient wraps the INSPIRE-HEP API.
type InspireClient struct {
	baseURL string
	token   string
	client  *resty.Client
	log     *zerolog.Logger
}

// NewInspireClient creates a new INSPIRE-HEP API client.
func NewInspireClient(baseURL, token string, log *zerolog.Logger) *InspireClient {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second)

	if token != "" {
		client.SetAuthToken(token)
	}

	return &InspireClient{
		baseURL: baseURL,
		token:   token,
		client:  client,
		log:     log,
	}
}

// SearchMeasurements searches INSPIRE-HEP for experimental measurements.
func (c *InspireClient) SearchMeasurements(ctx context.Context, observable string) ([]map[string]any, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		SetQueryParam("q", fmt.Sprintf("measurements.observable:%s and collaboration:CEPC", observable)).
		SetQueryParam("size", "10").
		SetQueryParam("format", "json").
		Get(c.baseURL + "/literature")
	if err != nil {
		return nil, fmt.Errorf("INSPIRE API request failed: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("INSPIRE API returned status %d", resp.StatusCode())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		return nil, fmt.Errorf("failed to parse INSPIRE JSON: %w", err)
	}

	results := make([]map[string]any, 0)
	if hits, ok := body["hits"].(map[string]any)["hits"].([]any); ok {
		for _, hit := range hits {
			if h, ok := hit.(map[string]any); ok {
				result := map[string]any{
					"inspire_id": h["id"],
					"title":      extractINSPIRETitle(h),
				}
				results = append(results, result)
			}
		}
	}

	c.log.Info().
		Str("observable", observable).
		Int("results", len(results)).
		Msg("INSPIRE search complete")

	return results, nil
}

func extractINSPIRETitle(hit map[string]any) string {
	if titles, ok := hit["titles"].([]any); ok && len(titles) > 0 {
		if t, ok := titles[0].(map[string]any)["title"].(string); ok {
			return t
		}
	}
	return "Unknown Title"
}
