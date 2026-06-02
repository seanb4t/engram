// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package embed produces text embeddings via an OpenAI-compatible
// /v1/embeddings endpoint (e.g. LiteLLM fronting Ollama bge-m3).
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client embeds text via an OpenAI-compatible embeddings API.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// New returns an embedding Client for the given base URL, API key, and model.
func New(baseURL, apiKey, model string) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: 30 * time.Second}}
}

type embedReq struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed returns the embedding vector for text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(embedReq{Model: c.model, Input: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings: status %d", resp.StatusCode)
	}
	var out embedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("embeddings: empty data")
	}
	return out.Data[0].Embedding, nil
}
