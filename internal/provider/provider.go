package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// Provider defines the interface for communicating with AI models.
type Provider interface {
	GenerateResponse(ctx context.Context, model string, prompt string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// ClientConfig holds configuration for the providers.
type ClientConfig struct {
	APIKey     string
	Endpoint   string
	Timeout    time.Duration
	APIVersion string // Azure OpenAI specific
}

// NewProvider creates the appropriate provider adapter.
func NewProvider(providerName string, config ClientConfig) (Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}

	switch providerName {
	case "openai":
		return &OpenAIProvider{cfg: config, client: httpClient}, nil
	case "claude":
		return &ClaudeProvider{cfg: config, client: httpClient}, nil
	case "gemini":
		return &GeminiProvider{cfg: config, client: httpClient}, nil
	case "ollama":
		if config.Endpoint == "" {
			config.Endpoint = "http://localhost:11434"
		}
		return &OllamaProvider{cfg: config, client: httpClient}, nil
	case "deepseek":
		return &DeepSeekProvider{cfg: config, client: httpClient}, nil
	case "azure":
		return &AzureProvider{cfg: config, client: httpClient}, nil
	case "mock":
		return &MockProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// ==========================================
// OpenAI Provider
// ==========================================
type OpenAIProvider struct {
	cfg    ClientConfig
	client *http.Client
}

func (p *OpenAIProvider) GenerateResponse(ctx context.Context, model string, prompt string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"
	if p.cfg.Endpoint != "" {
		url = p.cfg.Endpoint + "/v1/chat/completions"
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty choices returned from OpenAI")
	}
	return result.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	url := "https://api.openai.com/v1/embeddings"
	if p.cfg.Endpoint != "" {
		url = p.cfg.Endpoint + "/v1/embeddings"
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": "text-embedding-3-small",
		"input": text,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embedding error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("empty embedding returned from OpenAI")
	}
	return result.Data[0].Embedding, nil
}

// ==========================================
// Claude Provider (Anthropic)
// ==========================================
type ClaudeProvider struct {
	cfg    ClientConfig
	client *http.Client
}

func (p *ClaudeProvider) GenerateResponse(ctx context.Context, model string, prompt string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"
	if p.cfg.Endpoint != "" {
		url = p.cfg.Endpoint
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty content returned from Claude")
	}
	return result.Content[0].Text, nil
}

func (p *ClaudeProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Anthropic does not have a native embeddings API. Fall back to standard mock.
	mock := &MockProvider{}
	return mock.GenerateEmbedding(ctx, text)
}

// ==========================================
// Gemini Provider (Google)
// ==========================================
type GeminiProvider struct {
	cfg    ClientConfig
	client *http.Client
}

func (p *GeminiProvider) GenerateResponse(ctx context.Context, model string, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, p.cfg.APIKey)
	if p.cfg.Endpoint != "" {
		url = fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.cfg.Endpoint, model, p.cfg.APIKey)
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response returned from Gemini")
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}

func (p *GeminiProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=%s", p.cfg.APIKey)
	if p.cfg.Endpoint != "" {
		url = fmt.Sprintf("%s/v1beta/models/text-embedding-004:embedContent?key=%s", p.cfg.Endpoint, p.cfg.APIKey)
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"content": map[string]interface{}{
			"parts": []map[string]string{
				{"text": text},
			},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini embedding error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding.Values, nil
}

// ==========================================
// Ollama Provider (Local)
// ==========================================
type OllamaProvider struct {
	cfg    ClientConfig
	client *http.Client
}

func (p *OllamaProvider) GenerateResponse(ctx context.Context, model string, prompt string) (string, error) {
	url := p.cfg.Endpoint + "/api/generate"

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}

func (p *OllamaProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	url := p.cfg.Endpoint + "/api/embeddings"

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":  "nomic-embed-text",
		"prompt": text,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embedding error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

// ==========================================
// DeepSeek Provider
// ==========================================
type DeepSeekProvider struct {
	cfg    ClientConfig
	client *http.Client
}

func (p *DeepSeekProvider) GenerateResponse(ctx context.Context, model string, prompt string) (string, error) {
	url := "https://api.deepseek.com/v1/chat/completions"
	if p.cfg.Endpoint != "" {
		url = p.cfg.Endpoint
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
		"stream":      false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("deepseek error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response returned from DeepSeek")
	}
	return result.Choices[0].Message.Content, nil
}

func (p *DeepSeekProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// DeepSeek does not offer native embeddings. Fallback to mock.
	mock := &MockProvider{}
	return mock.GenerateEmbedding(ctx, text)
}

// ==========================================
// Azure OpenAI Provider
// ==========================================
type AzureProvider struct {
	cfg    ClientConfig
	client *http.Client
}

func (p *AzureProvider) GenerateResponse(ctx context.Context, model string, prompt string) (string, error) {
	// Azure requires endpoint structure: https://{resource-name}.openai.azure.com/openai/deployments/{deployment-id}/chat/completions?api-version={api-version}
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", p.cfg.Endpoint, model, p.cfg.APIVersion)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("azure openai error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response returned from Azure OpenAI")
	}
	return result.Choices[0].Message.Content, nil
}

func (p *AzureProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/openai/deployments/text-embedding-3-small/embeddings?api-version=%s", p.cfg.Endpoint, p.cfg.APIVersion)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"input": text,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure openai embedding error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data[0].Embedding, nil
}

// ==========================================
// Mock Provider (For Local Testing)
// ==========================================
type MockProvider struct{}

func (p *MockProvider) GenerateResponse(ctx context.Context, model string, prompt string) (string, error) {
	return fmt.Sprintf("[Mock Response for %s] I parsed your prompt. It contains %d characters.", model, len(prompt)), nil
}

func (p *MockProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Generate deterministic mock embedding of size 1536 based on string hash
	r := rand.New(rand.NewSource(int64(hashString(text))))
	vec := make([]float32, 1536)
	for i := 0; i < 1536; i++ {
		vec[i] = r.Float32()*2.0 - 1.0 // between -1 and 1
	}

	// Normalize vector
	sum := float32(0.0)
	for _, val := range vec {
		sum += val * val
	}
	norm := float32(math.Sqrt(float64(sum)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec, nil
}

func hashString(s string) uint32 {
	h := uint32(37)
	for i := 0; i < len(s); i++ {
		h = h*31 + uint32(s[i])
	}
	return h
}
