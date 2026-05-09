package agentctlsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Transport struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func NewTransport(endpoint, apiKey string, httpClient *http.Client) *Transport {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Transport{
		endpoint:   endpoint,
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

func (t *Transport) GetJSON(ctx context.Context, path string, query QueryParams, dst any) error {
	return t.requestJSON(ctx, http.MethodGet, path, query, nil, dst)
}

func (t *Transport) GetText(ctx context.Context, path string, query QueryParams) (string, error) {
	return t.requestText(ctx, http.MethodGet, path, query, nil, "*/*")
}

func (t *Transport) GetStream(ctx context.Context, path string, query QueryParams, onChunk func(string)) error {
	return t.requestStream(ctx, http.MethodGet, path, query, nil, onChunk)
}

func (t *Transport) PostJSON(ctx context.Context, path string, body any, dst any) error {
	return t.requestJSON(ctx, http.MethodPost, path, nil, body, dst)
}

func (t *Transport) PostText(ctx context.Context, path string, body any) (string, error) {
	return t.requestText(ctx, http.MethodPost, path, nil, body, "*/*")
}

func (t *Transport) PostStream(ctx context.Context, path string, body any, onChunk func(string)) error {
	return t.requestStream(ctx, http.MethodPost, path, nil, body, onChunk)
}

func (t *Transport) PutJSON(ctx context.Context, path string, body any, dst any) error {
	return t.requestJSON(ctx, http.MethodPut, path, nil, body, dst)
}

func (t *Transport) DeleteJSON(ctx context.Context, path string, query QueryParams, dst any) error {
	return t.requestJSON(ctx, http.MethodDelete, path, query, nil, dst)
}

func (t *Transport) WebSocket(ctx context.Context, path string, query QueryParams, initialMessage any, onMessage func(string)) error {
	wsURL, err := t.buildWebSocketURL(path, query)
	if err != nil {
		return err
	}

	headers := http.Header{}
	if t.apiKey != "" {
		headers.Set("Authorization", "Bearer "+t.apiKey)
	}

	if isDebugEnabled() {
		fmt.Fprintln(os.Stderr, "WS", wsURL)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return err
	}
	defer conn.Close()

	if initialMessage != nil {
		if err := conn.WriteJSON(initialMessage); err != nil {
			return err
		}
	}

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			return err
		}

		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		onMessage(string(data))
	}
}

func (t *Transport) requestJSON(ctx context.Context, method, path string, query QueryParams, body any, dst any) error {
	text, err := t.requestText(ctx, method, path, query, body, "application/json")
	if err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(text), dst); err != nil {
		preview := strings.Join(strings.Fields(text), " ")
		if len(preview) > 240 {
			preview = preview[:240]
		}
		return fmt.Errorf("expected JSON response, got: %s", preview)
	}
	return nil
}

func (t *Transport) requestText(ctx context.Context, method, path string, query QueryParams, body any, accept string) (string, error) {
	req, err := t.buildRequest(ctx, method, path, query, body, accept)
	if err != nil {
		return "", err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	text := string(raw)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%d: %s", resp.StatusCode, errorMessageFromResponse(text))
	}
	return text, nil
}

func (t *Transport) requestStream(ctx context.Context, method, path string, query QueryParams, body any, onChunk func(string)) error {
	req, err := t.buildRequest(ctx, method, path, query, body, "text/event-stream")
	if err != nil {
		return err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%d: %s", resp.StatusCode, errorMessageFromResponse(string(raw)))
	}

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			onChunk(string(buf[:n]))
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (t *Transport) buildRequest(ctx context.Context, method, path string, query QueryParams, body any, accept string) (*http.Request, error) {
	urlText, err := t.buildURL(path, query)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlText, reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", accept)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	if isDebugEnabled() {
		fmt.Fprintln(os.Stderr, method, urlText)
	}

	return req, nil
}

func (t *Transport) buildURL(path string, query QueryParams) (string, error) {
	base, err := url.Parse(t.endpoint)
	if err != nil {
		return "", err
	}

	basePath := base.Path
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	relativePath := strings.TrimLeft(path, "/")
	base.Path = strings.ReplaceAll(basePath+relativePath, "//", "/")

	values := base.Query()
	for key, value := range query {
		if isZeroValue(value) {
			continue
		}
		values.Set(key, fmt.Sprint(value))
	}
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func (t *Transport) buildWebSocketURL(path string, query QueryParams) (string, error) {
	urlText, err := t.buildURL(path, query)
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(urlText)
	if err != nil {
		return "", err
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	}

	return parsed.String(), nil
}

func isDebugEnabled() bool {
	return os.Getenv("AGENTCTL_DEBUG") == "1" || os.Getenv("SEAAGENT_DEBUG") == "1"
}

func errorMessageFromResponse(text string) string {
	if text == "" {
		return ""
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		if message, ok := parsed["error"]; ok {
			return fmt.Sprint(message)
		}
	}

	return text
}

func isZeroValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case int:
		return v == 0
	case int64:
		return v == 0
	case float64:
		return v == 0
	case bool:
		return false
	default:
		return false
	}
}
