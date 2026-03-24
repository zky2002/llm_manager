package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type llamaCPPProvider struct {
	url    string
	client *http.Client
}

func NewLlamaCPPProvider(baseURL string) Provider {
	return &llamaCPPProvider{
		url: strings.TrimSuffix(baseURL, "/"),
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type llamaReq struct {
	Prompt      string `json:"prompt"`
	NPredict    int    `json:"n_predict,omitempty"`
	Temperature any    `json:"temperature,omitempty"`
}

type llamaResp struct {
	Content string `json:"content"`
}

func (p *llamaCPPProvider) Generate(ctx context.Context, prompt string) (string, error) {
	body, _ := json.Marshal(llamaReq{Prompt: prompt, NPredict: 512})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url+"/completion", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llama.cpp status %d: %s", resp.StatusCode, string(raw))
	}

	var payload llamaResp
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Content), nil
}
