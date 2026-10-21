package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const validPlan = `{"commands":[{"command":"ls -l","risk":"safe"}],"explanation":"List files.","caveat":""}`

// recordingServer is a fake OpenAI-compatible server that keeps every
// request body it was sent.
type recordingServer struct {
	*httptest.Server
	requests []map[string]any
	// status is the status code to answer with. 0 means 200.
	status int
	// reply is the content to answer with.
	reply string
}

func newRecordingServer(t *testing.T) *recordingServer {
	t.Helper()
	rec := &recordingServer{reply: validPlan}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("server: cannot decode request: %v", err)
		}
		rec.requests = append(rec.requests, body)

		if rec.status != 0 && rec.status != http.StatusOK {
			w.WriteHeader(rec.status)
			_, _ = w.Write([]byte(`{"error":{"message":"unsupported"}}`))
			return
		}

		content := rec.reply
		response := map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"role": "assistant", "content": content},
				"finish_reason": "stop",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(rec.Close)
	return rec
}

func TestOpenAISendsTheSchema(t *testing.T) {
	server := newRecordingServer(t)
	planner := NewOpenAI(Config{BaseURL: server.URL + "/v1", Model: "test-model", System: "be helpful"})

	if _, err := planner.Plan(context.Background(), "list files"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(server.requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(server.requests))
	}

	format, ok := server.requests[0]["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("no response_format in request: %v", server.requests[0])
	}
	if format["type"] != "json_schema" {
		t.Errorf("response_format type = %v, want json_schema", format["type"])
	}
	jsonSchema, ok := format["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("no json_schema in response_format: %v", format)
	}
	if _, ok := jsonSchema["schema"].(map[string]any); !ok {
		t.Errorf("json_schema has no schema object: %v", jsonSchema)
	}
}

func TestOpenAIKeepsAssistantReplies(t *testing.T) {
	server := newRecordingServer(t)
	planner := NewOpenAI(Config{BaseURL: server.URL + "/v1", Model: "test-model", System: "be helpful"})

	if _, err := planner.Plan(context.Background(), "first request"); err != nil {
		t.Fatalf("first Plan: %v", err)
	}
	if _, err := planner.Plan(context.Background(), "now make it read-only"); err != nil {
		t.Fatalf("second Plan: %v", err)
	}
	if len(server.requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(server.requests))
	}

	messages, ok := server.requests[1]["messages"].([]any)
	if !ok {
		t.Fatalf("no messages in request: %v", server.requests[1])
	}
	var roles []string
	for _, m := range messages {
		msg, _ := m.(map[string]any)
		role, _ := msg["role"].(string)
		roles = append(roles, role)
	}

	want := []string{roleSystem, roleUser, roleAssistant, roleUser}
	if len(roles) != len(want) {
		t.Fatalf("roles = %v, want %v", roles, want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("roles = %v, want %v", roles, want)
		}
	}
}

func TestOpenAIFallsBackWhenSchemaIsRejected(t *testing.T) {
	// A server that rejects the schema but accepts the plain format.
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		format, _ := body["response_format"].(map[string]any)
		if format["type"] == "json_schema" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"unsupported"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": validPlan},
			}},
		})
	}))
	defer srv.Close()

	planner := NewOpenAI(Config{BaseURL: srv.URL + "/v1", Model: "test-model", System: "be helpful"})
	p, err := planner.Plan(context.Background(), "list files")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(p.Commands) != 1 || p.Commands[0].Command != "ls -l" {
		t.Errorf("plan = %+v, want one ls -l command", p)
	}
	if attempts != 2 {
		t.Errorf("got %d attempts, want 2 (schema then fallback)", attempts)
	}
}

func TestOpenAIDecodesProseAroundJSON(t *testing.T) {
	server := newRecordingServer(t)
	server.reply = "Sure! Here is the plan:\n" + validPlan + "\nHope that helps."

	planner := NewOpenAI(Config{BaseURL: server.URL + "/v1", Model: "test-model", System: "be helpful"})

	p, err := planner.Plan(context.Background(), "list files")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(p.Commands) != 1 {
		t.Fatalf("plan has %d commands, want 1", len(p.Commands))
	}
}

func TestOpenAIReportsHTTPErrors(t *testing.T) {
	server := newRecordingServer(t)
	server.status = http.StatusInternalServerError

	planner := NewOpenAI(Config{BaseURL: server.URL + "/v1", Model: "test-model", System: "be helpful"})

	_, err := planner.Plan(context.Background(), "list files")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q does not mention the status code", err)
	}
}

func TestOpenAIRequiresAModel(t *testing.T) {
	server := newRecordingServer(t)
	planner := NewOpenAI(Config{BaseURL: server.URL + "/v1", System: "be helpful"})

	_, err := planner.Plan(context.Background(), "list files")
	if err == nil {
		t.Fatal("expected an error when no model is configured")
	}
}
