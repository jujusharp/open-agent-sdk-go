package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jujusharp/open-agent-sdk-go/types"
)

func TestCreateOpenAIStreamKeepsToolArgumentsOnToolBlockAfterTextPrefix(t *testing.T) {
	argsJSON, err := json.Marshal(map[string]any{
		"command":     `grep -r "os.Getenv\|Getenv" /data/code/AirAgent/server/internal/config --include="*.go" 2>/dev/null`,
		"description": "Check if config reads from environment variables",
	})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected streaming response writer")
		}

		writeChunk := func(payload map[string]any) {
			t.Helper()
			data, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				t.Fatalf("marshal stream chunk: %v", marshalErr)
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		writeChunk(map[string]any{
			"id":     "chatcmpl_1",
			"object": "chat.completion.chunk",
			"model":  "gpt-5.4-mini",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{
					"content": "让我查看是否有从环境变量读取配置的逻辑：",
				},
				"finish_reason": nil,
			}},
		})

		writeChunk(map[string]any{
			"id":     "chatcmpl_1",
			"object": "chat.completion.chunk",
			"model":  "gpt-5.4-mini",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{
					"tool_calls": []map[string]any{{
						"index": 0,
						"id":    "call_1",
						"type":  "function",
						"function": map[string]any{
							"name":      "Bash",
							"arguments": string(argsJSON),
						},
					}},
				},
				"finish_reason": nil,
			}},
		})

		writeChunk(map[string]any{
			"id":     "chatcmpl_1",
			"object": "chat.completion.chunk",
			"model":  "gpt-5.4-mini",
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "tool_calls",
			}},
		})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		Provider:   ProviderOpenAI,
		HTTPClient: server.Client(),
	})
	eventCh := make(chan StreamEvent, 16)
	if err := client.createOpenAIStream(context.Background(), MessagesRequest{Model: "gpt-5.4-mini"}, eventCh); err != nil {
		t.Fatalf("createOpenAIStream: %v", err)
	}

	var toolStart *StreamEvent
	var toolDelta *StreamEvent
	for len(eventCh) > 0 {
		event := <-eventCh
		if event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == types.ContentBlockToolUse {
			captured := event
			toolStart = &captured
			continue
		}
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta["type"] == "input_json_delta" {
			captured := event
			toolDelta = &captured
		}
	}

	if toolStart == nil {
		t.Fatal("expected tool block start event")
	}
	if toolStart.Index != 1 {
		t.Fatalf("expected tool block to start after the text block at index 1, got %d", toolStart.Index)
	}
	if toolDelta == nil {
		t.Fatal("expected tool input delta event")
	}
	if toolDelta.Index != toolStart.Index {
		t.Fatalf("expected tool delta index %d, got %d", toolStart.Index, toolDelta.Index)
	}
}
