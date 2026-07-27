package literature

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Embedder generates vector embeddings using OpenAI's API.
type Embedder struct {
	apiKey  string
	model   string
	client  *http.Client
	log     *zerolog.Logger
}

// NewEmbedder creates a new OpenAI embedder.
func NewEmbedder(apiKey, model string, log *zerolog.Logger) *Embedder {
	return &Embedder{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
		log:    log,
	}
}

// Embed generates an embedding vector for the given text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.apiKey == "" {
		return make([]float32, 1536), nil // Placeholder for demo mode
	}

	body, _ := json.Marshal(map[string]any{
		"model": e.model,
		"input": text,
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts in a single request.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if e.apiKey == "" {
		result := make([][]float32, len(texts))
		for i := range result {
			result[i] = make([]float32, 1536)
		}
		return result, nil
	}

	body, _ := json.Marshal(map[string]any{
		"model": e.model,
		"input": texts,
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}
	return embeddings, nil
}

// IsEnabled returns true if the embedder has a valid API key.
func (e *Embedder) IsEnabled() bool {
	return e.apiKey != "" && !strings.HasPrefix(e.apiKey, "sk-") == false
}
