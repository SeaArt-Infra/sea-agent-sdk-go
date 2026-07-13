package seaagentsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Transport struct {
	endpoint         string
	apiKey           string
	headers          map[string]string
	httpClient       *http.Client
	streamHTTPClient *http.Client
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%d: %s", e.StatusCode, e.Message)
}

func NewTransport(endpoint, apiKey string, headers map[string]string, httpClient *http.Client) *Transport {
	streamHTTPClient := httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
		streamHTTPClient = &http.Client{}
	}
	return &Transport{
		endpoint:         normalizeAgentGatewayEndpoint(endpoint),
		apiKey:           apiKey,
		headers:          cloneStringMap(headers),
		httpClient:       httpClient,
		streamHTTPClient: streamHTTPClient,
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

func (t *Transport) PostJSONWithHeaders(ctx context.Context, path string, body any, headers map[string]string, dst any) error {
	return t.requestJSONWithHeaders(ctx, http.MethodPost, path, nil, body, headers, dst)
}

func (t *Transport) PostText(ctx context.Context, path string, body any) (string, error) {
	return t.requestText(ctx, http.MethodPost, path, nil, body, "*/*")
}

func (t *Transport) PostStream(ctx context.Context, path string, body any, onChunk func(string)) error {
	return t.requestStream(ctx, http.MethodPost, path, nil, body, onChunk)
}

func (t *Transport) PostStreamWithHeaders(ctx context.Context, path string, body any, headers map[string]string, onChunk func(string)) error {
	return t.requestStreamWithHeaders(ctx, http.MethodPost, path, nil, body, headers, onChunk)
}

func (t *Transport) PutJSON(ctx context.Context, path string, body any, dst any) error {
	return t.requestJSON(ctx, http.MethodPut, path, nil, body, dst)
}

func (t *Transport) DeleteJSON(ctx context.Context, path string, query QueryParams, dst any) error {
	return t.requestJSON(ctx, http.MethodDelete, path, query, nil, dst)
}

func (t *Transport) WebSocket(ctx context.Context, path string, query QueryParams, initialMessage any, onMessage func(string)) error {
	return t.WebSocketWithHeaders(ctx, path, query, initialMessage, nil, onMessage)
}

func (t *Transport) WebSocketWithHeaders(ctx context.Context, path string, query QueryParams, initialMessage any, headers map[string]string, onMessage func(string)) error {
	return t.webSocketWithHeaders(ctx, path, query, initialMessage, headers, func(message string) error {
		onMessage(message)
		return nil
	})
}

func (t *Transport) webSocketWithHeaders(ctx context.Context, path string, query QueryParams, initialMessage any, headers map[string]string, onMessage func(string) error) error {
	wsURL, err := t.buildWebSocketURL(path, query)
	if err != nil {
		return err
	}

	requestHeaders := t.buildHeaders("*/*", false, headers)

	if isDebugEnabled() {
		fmt.Fprintln(os.Stderr, "WS", wsURL)
	}

	conn, response, err := websocket.DefaultDialer.DialContext(ctx, wsURL, requestHeaders)
	if err != nil {
		if response != nil {
			defer func() { _ = response.Body.Close() }()
			raw, _ := io.ReadAll(response.Body)
			return &HTTPError{StatusCode: response.StatusCode, Message: errorMessageFromResponse(string(raw))}
		}
		return err
	}
	defer func() { _ = conn.Close() }()
	stopClose := make(chan struct{})
	defer close(stopClose)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stopClose:
		}
	}()

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

		if err := onMessage(string(data)); err != nil {
			return err
		}
	}
}

func (t *Transport) requestJSON(ctx context.Context, method, path string, query QueryParams, body any, dst any) error {
	return t.requestJSONWithHeaders(ctx, method, path, query, body, nil, dst)
}

func (t *Transport) requestJSONWithHeaders(ctx context.Context, method, path string, query QueryParams, body any, headers map[string]string, dst any) error {
	text, err := t.requestTextWithHeaders(ctx, method, path, query, body, "application/json", headers)
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
	return t.requestTextWithHeaders(ctx, method, path, query, body, accept, nil)
}

func (t *Transport) requestTextWithHeaders(ctx context.Context, method, path string, query QueryParams, body any, accept string, headers map[string]string) (string, error) {
	req, err := t.buildRequestWithHeaders(ctx, method, path, query, body, accept, headers)
	if err != nil {
		return "", err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	text := string(raw)
	if resp.StatusCode >= 400 {
		return "", &HTTPError{StatusCode: resp.StatusCode, Message: errorMessageFromResponse(text)}
	}
	return text, nil
}

func (t *Transport) requestStream(ctx context.Context, method, path string, query QueryParams, body any, onChunk func(string)) error {
	return t.requestStreamWithHeaders(ctx, method, path, query, body, nil, onChunk)
}

func (t *Transport) requestStreamWithHeaders(ctx context.Context, method, path string, query QueryParams, body any, headers map[string]string, onChunk func(string)) error {
	return t.requestStreamWithHeadersCallback(ctx, method, path, query, body, headers, func(chunk string) error {
		onChunk(chunk)
		return nil
	})
}

func (t *Transport) requestStreamWithHeadersCallback(ctx context.Context, method, path string, query QueryParams, body any, headers map[string]string, onChunk func(string) error) error {
	req, err := t.buildRequestWithHeaders(ctx, method, path, query, body, "text/event-stream", headers)
	if err != nil {
		return err
	}

	resp, err := t.streamHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Message: errorMessageFromResponse(string(raw))}
	}

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if callbackErr := onChunk(string(buf[:n])); callbackErr != nil {
				return callbackErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (t *Transport) buildRequestWithHeaders(ctx context.Context, method, path string, query QueryParams, body any, accept string, headers map[string]string) (*http.Request, error) {
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

	req.Header = t.buildHeaders(accept, body != nil, headers)

	if isDebugEnabled() {
		fmt.Fprintln(os.Stderr, method, urlText)
	}

	return req, nil
}

func (t *Transport) buildHeaders(accept string, hasBody bool, requestHeaders map[string]string) http.Header {
	headers := http.Header{}
	if accept != "" {
		headers.Set("Accept", accept)
	}
	if hasBody {
		headers.Set("Content-Type", "application/json")
	}
	for key, value := range t.headers {
		if strings.TrimSpace(key) != "" {
			headers.Set(key, value)
		}
	}
	for key, value := range requestHeaders {
		if strings.TrimSpace(key) != "" {
			headers.Set(key, value)
		}
	}
	if t.apiKey != "" && headers.Get("Authorization") == "" {
		headers.Set("Authorization", "Bearer "+t.apiKey)
	}
	return headers
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
		if boolValue, ok := value.(*bool); ok {
			values.Set(key, strconv.FormatBool(*boolValue))
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

func normalizeAgentGatewayEndpoint(endpoint string) string {
	if strings.TrimSpace(endpoint) == "" {
		return endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}

	segments := strings.FieldsFunc(parsed.Path, func(r rune) bool {
		return r == '/'
	})
	for _, segment := range segments {
		if segment == "agent-v2" {
			return parsed.String()
		}
	}
	segments = append(segments, "agent-v2")
	parsed.Path = "/" + strings.Join(segments, "/")
	return parsed.String()
}

func isDebugEnabled() bool {
	return os.Getenv("SEAAGENT_DEBUG") == "1"
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

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
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
	case *bool:
		return v == nil
	default:
		return false
	}
}
