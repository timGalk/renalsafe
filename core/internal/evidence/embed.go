package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Dimensions = 1024

type Embedder interface {
	Model() string
	Embed(context.Context, []string) ([][]float64, error)
}
type Ollama struct {
	BaseURL, ModelName string
	Client             *http.Client
}

func NewOllama(base, model string) (*Ollama, error) {
	u, e := url.Parse(base)
	if e != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || model == "" {
		return nil, fmt.Errorf("invalid embedding provider configuration")
	}
	// Local-only contract: no patient or source data is sent to a public model endpoint.
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "::1" && host != "ollama" && host != "host.docker.internal" {
		return nil, fmt.Errorf("embedding provider must be local")
	}
	return &Ollama{BaseURL: strings.TrimRight(base, "/"), ModelName: model, Client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (o *Ollama) Model() string { return o.ModelName }
func ValidVector(v []float64) bool {
	if len(v) != Dimensions {
		return false
	}
	norm := 0.0
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e10 {
			return false
		}
		norm += x * x
	}
	return norm > 0
}
func (o *Ollama) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	b, _ := json.Marshal(map[string]any{"model": o.ModelName, "input": texts, "truncate": false})
	req, e := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/embed", bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := o.Client.Do(req)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("embedding provider status %d", res.StatusCode)
	}
	var data struct {
		Model      string      `json:"model"`
		Embeddings [][]float64 `json:"embeddings"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&data); e != nil {
		return nil, e
	}
	if data.Model != o.ModelName || len(data.Embeddings) != len(texts) {
		return nil, fmt.Errorf("embedding model or count mismatch")
	}
	for _, v := range data.Embeddings {
		if !ValidVector(v) {
			return nil, fmt.Errorf("embedding dimensions or values invalid")
		}
	}
	return data.Embeddings, nil
}
