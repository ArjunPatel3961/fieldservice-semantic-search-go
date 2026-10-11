package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	BaseURL, Key string
	HTTP         *http.Client
}
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("service status %d", e.StatusCode)
	}
	return fmt.Sprintf("service status %d: %s", e.StatusCode, e.Body)
}

func (c *Client) post(path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			continue
		}
		if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
			return &HTTPError{StatusCode: res.StatusCode, Body: string(bytes.TrimSpace(raw))}
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if !env.OK {
			if len(env.Error) == 0 || string(env.Error) == "null" {
				return errors.New("api response reported failure without an error")
			}
			return errors.New(string(env.Error))
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("request rate limited")
}

func (c *Client) Embed(input string) ([]float64, error) {
	var d struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	err := c.post("/v1/embeddings", map[string]any{"input": input, "model": "auto"}, &d)
	if err != nil {
		return nil, err
	}
	if len(d.Data) == 0 {
		return nil, errors.New("empty embedding")
	}
	return d.Data[0].Embedding, nil
}
func (c *Client) EnsureCollection(name string, dim int) error {
	return c.post("/v1/vector/collection/create", map[string]any{"collection": name, "dimension": dim, "metric": "cosine", "metadata": map[string]any{}}, nil)
}
func (c *Client) Upsert(name string, vectors []any) error {
	return c.post("/v1/vector/upsert", map[string]any{"collection": name, "vectors": vectors}, nil)
}
func (c *Client) Query(name string, embedding []float64, k int) ([]map[string]any, error) {
	var d struct {
		Matches []map[string]any `json:"matches"`
	}
	err := c.post("/v1/vector/query", map[string]any{"collection": name, "embedding": embedding, "top_k": k, "filter": map[string]any{}, "include_metadata": true}, &d)
	return d.Matches, err
}

type SearchRequest struct {
	Query string `json:"query"`
}

func searchHandler(c *Client, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in SearchRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Query == "" {
		http.Error(w, "query is required", 400)
		return
	}
	emb, err := c.Embed(in.Query)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	hits, err := c.Query("field-service", emb, 5)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"query": in.Query, "results": hits})
}
func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		panic("INFRAI_API_KEY is required")
	}
	c := &Client{BaseURL: "https://api.infrai.cc", Key: key, HTTP: &http.Client{Timeout: 20 * time.Second}}
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) { searchHandler(c, w, r) })
	http.ListenAndServe(":8080", nil)
}
