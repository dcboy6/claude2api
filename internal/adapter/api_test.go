package adapter

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"claude2api/internal/service"

	"github.com/gin-gonic/gin"
)

type testRunner func(string, service.Prompt, func(string)) (service.CompletionResult, error)

func (f testRunner) Complete(model string, prompt service.Prompt, emit func(string)) (service.CompletionResult, error) {
	return f(model, prompt, emit)
}

func TestAPIContracts(t *testing.T) {
	old := runner
	oldFetch := fetchImage
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	fetchImage = func(string) ([]byte, error) { return png, nil }
	runner = testRunner(func(_ string, prompt service.Prompt, emit func(string)) (service.CompletionResult, error) {
		if len(prompt.RawRequest) == 0 {
			t.Error("应保留原始请求 JSON")
		}
		if strings.Contains(string(prompt.RawRequest), `"image`) && len(prompt.Images) == 0 {
			t.Error("应归一化图片输入")
		}
		text := "ok"
		if strings.Contains(prompt.Text, "tool_result_json") {
			text = "<final_answer>done</final_answer>"
		} else if strings.Contains(prompt.Text, "Available tools") {
			text = `<tool_calls>[{"name":"weather","arguments":{"city":"上海"}}]</tool_calls>`
		}
		emit(text)
		return service.CompletionResult{StatusCode: http.StatusOK}, nil
	})
	defer func() { runner, fetchImage = old, oldFetch }()

	tool := `"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}]`
	cases := []struct {
		name, method, body, want string
		handler                  gin.HandlerFunc
	}{
		{"models", "GET", "", "claude-sonnet-4-6", ListModels},
		{"models sonnet 5.5", "GET", "", "claude-sonnet-5-5", ListModels},
		{"models opus 5.5", "GET", "", "claude-opus-5-5", ListModels},
		{"models 5.5 thinking", "GET", "", "claude-opus-5-5-thinking", ListModels},
		{"chat single", "POST", `{"messages":[{"role":"user","content":"hi"}]}`, `"chat.completion"`, OpenAIChat},
		{"chat multi image stream", "POST", `{"stream":true,"messages":[{"role":"user","content":"first"},{"role":"assistant","content":"answer"},{"role":"user","content":[{"type":"text","text":"image"},{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}}]}]}`, "data: [DONE]", OpenAIChat},
		{"chat image url", "POST", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]}]}`, `"chat.completion"`, OpenAIChat},
		{"chat raw base64", "POST", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}]}]}`, `"chat.completion"`, OpenAIChat},
		{"responses", "POST", `{"input":"hi"}`, `"object":"response"`, OpenAIResponses},
		{"responses image stream", "POST", `{"stream":true,"input":[{"role":"user","content":[{"type":"input_text","text":"hi"},{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}]}]}`, "response.completed", OpenAIResponses},
		{"responses image url", "POST", `{"input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.com/image.png"}]}]}`, `"object":"response"`, OpenAIResponses},
		{"messages", "POST", `{"messages":[{"role":"user","content":"hi"}]}`, `"type":"message"`, AnthropicMessages},
		{"messages image stream", "POST", `{"stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}}]}]}`, "message_stop", AnthropicMessages},
		{"messages image url", "POST", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"url","url":"https://example.com/image.png"}}]}]}`, `"type":"message"`, AnthropicMessages},
		{"chat tool", "POST", `{"messages":[{"role":"user","content":"weather"}],` + tool + `}`, "tool_calls", OpenAIChat},
		{"responses tool", "POST", `{"input":"weather",` + tool + `}`, "function_call", OpenAIResponses},
		{"messages tool", "POST", `{"messages":[{"role":"user","content":"weather"}],` + tool + `}`, "tool_use", AnthropicMessages},
		{"chat tool stream", "POST", `{"stream":true,"messages":[{"role":"user","content":"weather"}],` + tool + `}`, "tool_calls", OpenAIChat},
		{"responses tool stream", "POST", `{"stream":true,"input":"weather",` + tool + `}`, "response.function_call_arguments.done", OpenAIResponses},
		{"messages tool stream", "POST", `{"stream":true,"messages":[{"role":"user","content":"weather"}],` + tool + `}`, `"stop_reason":"tool_use"`, AnthropicMessages},
		{"chat tool result", "POST", `{"messages":[{"role":"user","content":"weather"},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"weather","arguments":"{\"city\":\"上海\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"sunny"}],` + tool + `}`, "done", OpenAIChat},
		{"responses tool result", "POST", `{"input":[{"role":"user","content":"weather"},{"type":"function_call","call_id":"call_1","name":"weather","arguments":"{\"city\":\"上海\"}"},{"type":"function_call_output","call_id":"call_1","output":"sunny"}],` + tool + `}`, "done", OpenAIResponses},
		{"messages tool result", "POST", `{"messages":[{"role":"user","content":"weather"},{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"weather","input":{"city":"上海"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"sunny"}]}],` + tool + `}`, "done", AnthropicMessages},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tc.method, "/v1", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			tc.handler(c)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestListModelsIncludesClaude55Family(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)
	ListModels(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, id := range []string{
		"claude-sonnet-4-6", "claude-sonnet-4-6-thinking",
		"claude-haiku-4-5-20251001", "claude-sonnet-5",
		"claude-sonnet-5-5", "claude-sonnet-5-5-thinking",
		"claude-opus-5-5", "claude-opus-5-5-thinking",
	} {
		if !strings.Contains(body, `"id":"`+id+`"`) {
			t.Fatalf("GET /v1/models missing %s: %s", id, body)
		}
	}
}

func TestModelAliasForwarding(t *testing.T) {
	old := runner
	t.Cleanup(func() { runner = old })

	cases := []struct {
		name, body, want string
		handler          gin.HandlerFunc
	}{
		{"chat sonnet 5.5 alias", `{"model":"claude-sonnet-5.5-thinking","messages":[{"role":"user","content":"hi"}]}`, "claude-sonnet-5-5-thinking", OpenAIChat},
		{"responses opus 5.5 alias", `{"model":"opus-5-5","input":"hi"}`, "claude-opus-5-5", OpenAIResponses},
		{"messages sonnet 5.5 dated", `{"model":"claude-sonnet-5-5-20260928","messages":[{"role":"user","content":"hi"}]}`, "claude-sonnet-5-5", AnthropicMessages},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			runner = testRunner(func(model string, _ service.Prompt, emit func(string)) (service.CompletionResult, error) {
				got = model
				emit("ok")
				return service.CompletionResult{StatusCode: http.StatusOK}, nil
			})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			tc.handler(c)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if got != tc.want {
				t.Fatalf("runner model=%q want %q", got, tc.want)
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("response should echo canonical model %s: %s", tc.want, w.Body.String())
			}
		})
	}
}

func TestUpstreamStatus(t *testing.T) {
	old := runner
	runner = testRunner(func(string, service.Prompt, func(string)) (service.CompletionResult, error) {
		err := errors.New("rate limited")
		return service.CompletionResult{StatusCode: http.StatusTooManyRequests}, &service.CompletionError{StatusCode: http.StatusTooManyRequests, Err: err}
	})
	defer func() { runner = old }()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	OpenAIChat(c)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAnthropicCountTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-sonnet-5-5","messages":[{"role":"user","content":"hello world"}]}`))
	AnthropicCountTokens(c)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"input_tokens"`) {
		t.Fatalf("unexpected response %d %s", w.Code, w.Body.String())
	}
}
