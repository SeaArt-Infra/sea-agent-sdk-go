package seaagentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestParseSSEExposesEventIDAndSequence(t *testing.T) {
	events := ParseSSE("event: chat.delta\r\ndata: {\"content\":\"hello\"}\r\nid: 42\r\n\r\nevent: heartbeat\r\ndata: {}\r\n\r\n")
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].ID != "42" || events[0].Seq != 42 {
		t.Fatalf("first event cursor = (%q, %d), want (42, 42)", events[0].ID, events[0].Seq)
	}
	if events[1].ID != "" || events[1].Seq != 0 {
		t.Fatalf("heartbeat cursor = (%q, %d), want empty and zero", events[1].ID, events[1].Seq)
	}
}

func TestParseWebSocketEventExposesEventIDAndSequence(t *testing.T) {
	event, err := ParseWebSocketEvent(`{"id":"9007199254740993","event":"chat.delta","data":{"content":"hello"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != "9007199254740993" || event.Seq != 9007199254740993 {
		t.Fatalf("event cursor = (%q, %d)", event.ID, event.Seq)
	}
}

func TestChatStreamProcessorTracksStateAndDeduplicates(t *testing.T) {
	var seqs []int64
	var textSeqs []int64
	processor := NewChatStreamProcessor(ChatStreamHandlers{
		OnEvent: func(event ChatStreamEvent) {
			seqs = append(seqs, event.Seq)
		},
		OnTextDelta: func(_ string, event ChatStreamEvent) {
			textSeqs = append(textSeqs, event.Seq)
		},
	})
	processor.WriteSSEChunk("event: chat.created\ndata: {\"run_id\":\"run_1\"}\nid: 1\n\n")
	processor.WriteSSEChunk("event: chat.delta\ndata: {\"content\":\"hel\"}\nid: 2\n\n")
	processor.WriteSSEChunk("event: chat.delta\ndata: {\"content\":\"duplicate\"}\nid: 2\n\n")
	processor.WriteSSEChunk("event: chat.delta\ndata: {\"text\":\"lo\"}\nid: 3\n\n")
	processor.WriteSSEChunk("event: response.completed\ndata: {}\nid: 4\n\n")

	if processor.RunID != "run_1" || processor.LastSeq != 4 || !processor.Terminal {
		t.Fatalf("processor state = run %q, seq %d, terminal %t", processor.RunID, processor.LastSeq, processor.Terminal)
	}
	if got := processor.Text(); got != "hello" {
		t.Fatalf("text = %q, want hello", got)
	}
	if want := []int64{1, 2, 3, 4}; !reflect.DeepEqual(seqs, want) {
		t.Fatalf("delivered seqs = %v, want %v", seqs, want)
	}
	if want := []int64{2, 3}; !reflect.DeepEqual(textSeqs, want) {
		t.Fatalf("text callback seqs = %v, want %v", textSeqs, want)
	}
}

func TestChatStreamProcessorRecognizesTerminalAliases(t *testing.T) {
	for _, eventName := range []string{"chat.completed", "response.canceled"} {
		var delivered []string
		processor := NewChatStreamProcessor(ChatStreamHandlers{})
		processor.handlers.OnEvent = func(event ChatStreamEvent) {
			delivered = append(delivered, event.Event)
		}
		processor.WriteSSEChunk(fmt.Sprintf("event: %s\ndata: {}\nid: 1\n\nevent: chat.delta\ndata: {\"content\":\"ignored\"}\nid: 2\n\n", eventName))
		if !processor.Terminal {
			t.Fatalf("event %q was not recognized as terminal", eventName)
		}
		if got := processor.Text(); got != "" {
			t.Fatalf("event after %q produced text %q", eventName, got)
		}
		if want := []string{eventName}; !reflect.DeepEqual(delivered, want) {
			t.Fatalf("events delivered after terminal = %v, want %v", delivered, want)
		}
	}
}

func TestRunStreamStopsReadingWhenTerminalConnectionStaysOpen(t *testing.T) {
	handlerDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: chat.created\ndata: {\"run_id\":\"run_terminal\"}\nid: 1\n\n")
		_, _ = fmt.Fprint(w, "event: chat.completed\ndata: {}\nid: 2\n\n")
		w.(http.Flusher).Flush()
		<-req.Context().Done()
		close(handlerDone)
	}))
	t.Cleanup(server.Close)

	result := make(chan error, 1)
	go func() {
		_, err := newTestClient(server).Chat.RunStream(context.Background(), ChatRunOptions{Message: "hello"}, ChatStreamHandlers{})
		result <- err
	}()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunStream did not return after terminal event")
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("stream response was not closed after terminal event")
	}
}

func TestRunStreamWebSocketStopsReadingWhenTerminalConnectionStaysOpen(t *testing.T) {
	handlerDone := make(chan struct{})
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Errorf("read initial request: %v", err)
			return
		}
		if err := conn.WriteJSON(map[string]any{"id": "1", "event": "chat.created", "data": map[string]any{"run_id": "run_terminal_ws"}}); err != nil {
			t.Errorf("write created event: %v", err)
			return
		}
		if err := conn.WriteJSON(map[string]any{"id": "2", "event": "response.canceled", "data": map[string]any{}}); err != nil {
			t.Errorf("write terminal event: %v", err)
			return
		}
		_, _, _ = conn.ReadMessage()
		close(handlerDone)
	}))
	t.Cleanup(server.Close)

	result := make(chan error, 1)
	go func() {
		_, err := newTestClient(server).Chat.RunStream(context.Background(), ChatRunOptions{Message: "hello"}, ChatStreamHandlers{Transport: StreamTransportWS})
		result <- err
	}()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket RunStream did not return after terminal event")
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket connection was not closed after terminal event")
	}
}

func TestRunStreamResumesByRunIDAndAfterSeq(t *testing.T) {
	var postCalls, replayCalls int
	var replayAfterSeq string
	var requestID string
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/agent-v2/v1/chat/completions":
			postCalls++
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
			}
			requestID, _ = body["request_id"].(string)
			_, _ = fmt.Fprint(w, "event: chat.created\ndata: {\"run_id\":\"run_1\"}\nid: 1\n\n")
			_, _ = fmt.Fprint(w, "event: response.text.delta\ndata: {\"delta\":\"hel\"}\nid: 2\n\n")
			_, _ = fmt.Fprint(w, "event: response.text.delta\ndata: {\"delta\":\"discarded\"}\nid: 3")
		case req.Method == http.MethodGet && req.URL.Path == "/agent-v2/v1/chats/run_1/stream":
			replayCalls++
			replayAfterSeq = req.URL.Query().Get("after_seq")
			receivedHeader = req.Header.Get("X-Stream-Header")
			_, _ = fmt.Fprint(w, "event: response.text.delta\ndata: {\"delta\":\"duplicate\"}\nid: 2\n\n")
			_, _ = fmt.Fprint(w, "event: chat.delta\ndata: {\"content\":\"lo\"}\nid: 3\n\n")
			_, _ = fmt.Fprint(w, "event: response.completed\ndata: {}\nid: 4\n\n")
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)

	var delivered []int64
	client := newTestClient(server)
	text, err := client.Chat.RunStream(context.Background(), ChatRunOptions{
		AgentID: "agent_1",
		Message: "hello",
		Headers: map[string]string{"X-Stream-Header": "kept"},
	}, ChatStreamHandlers{
		ReconnectDelay:    time.Nanosecond,
		MaxReconnectDelay: time.Nanosecond,
		OnEvent: func(event ChatStreamEvent) {
			delivered = append(delivered, event.Seq)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Fatalf("text = %q, want hello", text)
	}
	if postCalls != 1 || replayCalls != 1 || replayAfterSeq != "2" {
		t.Fatalf("calls = POST %d GET %d after_seq %q", postCalls, replayCalls, replayAfterSeq)
	}
	if requestID == "" {
		t.Fatal("generated request_id is empty")
	}
	if receivedHeader != "kept" {
		t.Fatalf("replay header = %q, want kept", receivedHeader)
	}
	if want := []int64{1, 2, 3, 4}; !reflect.DeepEqual(delivered, want) {
		t.Fatalf("delivered seqs = %v, want %v", delivered, want)
	}
}

func TestRunStreamRetriesPostWithStableRequestIDBeforeRunID(t *testing.T) {
	var calls int
	var requestIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requestIDs = append(requestIDs, fmt.Sprint(body["request_id"]))
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			_, _ = fmt.Fprint(w, "event: response.text.delta\ndata: {\"delta\":\"a\"}\nid: 1\n\n")
			return
		}
		_, _ = fmt.Fprint(w, "event: response.text.delta\ndata: {\"delta\":\"duplicate\"}\nid: 1\n\n")
		_, _ = fmt.Fprint(w, "event: chat.created\ndata: {\"run_id\":\"run_2\"}\nid: 2\n\n")
		_, _ = fmt.Fprint(w, "event: chat.delta\ndata: {\"delta\":\"b\"}\nid: 3\n\n")
		_, _ = fmt.Fprint(w, "event: response.completed\ndata: {}\nid: 4\n\n")
	}))
	t.Cleanup(server.Close)

	text, err := newTestClient(server).Chat.RunStream(context.Background(), ChatRunOptions{Message: "hello"}, ChatStreamHandlers{
		ReconnectDelay:    time.Nanosecond,
		MaxReconnectDelay: time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "ab" {
		t.Fatalf("text = %q, want ab", text)
	}
	if calls != 2 || len(requestIDs) != 2 || requestIDs[0] == "" || requestIDs[0] != requestIDs[1] {
		t.Fatalf("request IDs = %v across %d calls", requestIDs, calls)
	}
}

func TestStreamExistingChatAutomaticallyResumes(t *testing.T) {
	var afterSeqs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		afterSeqs = append(afterSeqs, req.URL.Query().Get("after_seq"))
		w.Header().Set("Content-Type", "text/event-stream")
		if len(afterSeqs) == 1 {
			_, _ = fmt.Fprint(w, "event: chat.delta\ndata: {\"content\":\"a\"}\nid: 6\n\n")
			return
		}
		_, _ = fmt.Fprint(w, "event: chat.response\ndata: {\"content\":\"b\"}\nid: 7\n\n")
	}))
	t.Cleanup(server.Close)

	text, err := newTestClient(server).Chat.Stream(context.Background(), "run existing", ChatStreamHandlers{
		ReconnectDelay:    time.Nanosecond,
		MaxReconnectDelay: time.Nanosecond,
	}, ChatEventsOptions{AfterSeq: 5})
	if err != nil {
		t.Fatal(err)
	}
	if text != "ab" || !reflect.DeepEqual(afterSeqs, []string{"5", "6"}) {
		t.Fatalf("text = %q, after_seq = %v", text, afterSeqs)
	}
}

func TestRunStreamCanDisableAutoResume(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: chat.delta\ndata: {\"content\":\"partial\"}\nid: 1\n\n")
	}))
	t.Cleanup(server.Close)

	text, err := newTestClient(server).Chat.RunStream(context.Background(), ChatRunOptions{Message: "hello"}, ChatStreamHandlers{DisableAutoResume: true})
	if err != nil {
		t.Fatal(err)
	}
	if text != "partial" || calls != 1 {
		t.Fatalf("text = %q, calls = %d", text, calls)
	}
}

func TestRunStreamHonorsReconnectLimit(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(server).Chat.RunStream(context.Background(), ChatRunOptions{Message: "hello"}, ChatStreamHandlers{
		MaxReconnects:     1,
		ReconnectDelay:    time.Nanosecond,
		MaxReconnectDelay: time.Nanosecond,
	})
	if err == nil || !strings.Contains(err.Error(), "after 1 reconnects") {
		t.Fatalf("error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestRunStreamStopsReconnectWhenContextIsCancelled(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	_, err := newTestClient(server).Chat.RunStream(ctx, ChatRunOptions{Message: "hello"}, ChatStreamHandlers{
		ReconnectDelay: time.Hour,
		OnReconnect: func(ChatStreamReconnectInfo) {
			cancel()
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRunStreamDoesNotRetryNonTransientHTTPError(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(server).Chat.RunStream(context.Background(), ChatRunOptions{Message: "hello"}, ChatStreamHandlers{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %v, want HTTP 400", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRunStreamReturnsWebSocketProcessingErrorWithoutRetry(t *testing.T) {
	var calls int
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Errorf("read initial request: %v", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"error","code":"bad_event","error":"boom"}`)); err != nil {
			t.Errorf("write error event: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(server).Chat.RunStream(context.Background(), ChatRunOptions{Message: "hello"}, ChatStreamHandlers{Transport: StreamTransportWS})
	if err == nil || !strings.Contains(err.Error(), "bad_event: boom") {
		t.Fatalf("error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func newTestClient(server *httptest.Server) *Client {
	endpoint, _ := url.JoinPath(server.URL, "agent-v2")
	return NewClient(ClientOptions{Endpoint: endpoint, HTTPClient: server.Client()})
}
