package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jonah/nlsh/internal/plan"
)

// DefaultTimeout bounds a single request to the model.
const DefaultTimeout = 180 * time.Second

// Config describes an OpenAI-compatible endpoint.
type Config struct {
	// BaseURL is the root of the API, for example
	// http://localhost:11434/v1.
	BaseURL string
	// APIKey is sent as a bearer token when it is not empty.
	APIKey string
	// Model is the model name to ask for.
	Model string
	// System is the instruction that describes the task.
	System string
}

// OpenAI talks to any server that speaks the OpenAI chat-completions API:
// OpenAI itself, Ollama, LM Studio, or apfel.
type OpenAI struct {
	cfg    Config
	client *http.Client

	// history holds the conversation so far, including the system
	// message and every assistant reply. Both matter: without the
	// assistant replies the model cannot see what it proposed last time
	// and cannot act on a follow-up such as "now make it read-only".
	history []message
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
)

// NewOpenAI returns a planner for an OpenAI-compatible endpoint.
func NewOpenAI(cfg Config) *OpenAI {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	return &OpenAI{
		cfg:     cfg,
		client:  &http.Client{Timeout: DefaultTimeout},
		history: []message{{Role: roleSystem, Content: cfg.System}},
	}
}

// Plan asks the model for a plan.
func (o *OpenAI) Plan(ctx context.Context, request string) (plan.Plan, error) {
	raw, err := o.chat(ctx, request)
	if err != nil {
		return plan.Plan{}, err
	}
	p, err := plan.Decode(raw)
	if err != nil {
		// Some servers ignore the response format and answer with
		// prose around the JSON. Recover the object if there is one.
		if obj, ok := plan.ExtractJSON(raw); ok {
			if p2, err2 := plan.Decode(obj); err2 == nil {
				return p2, nil
			}
		}
		// Otherwise ask for the same plan again, with a reminder.
		repaired, rerr := o.chat(ctx, repairInstruction)
		if rerr != nil {
			return plan.Plan{}, fmt.Errorf("%w (and the repair attempt failed: %v)", err, rerr)
		}
		obj, ok := plan.ExtractJSON(repaired)
		if !ok {
			return plan.Plan{}, fmt.Errorf("the model did not return a usable plan")
		}
		return plan.Decode(obj)
	}
	return p, nil
}

// Regenerate asks the model to fix a plan that failed validation.
func (o *OpenAI) Regenerate(ctx context.Context, feedback string) (plan.Plan, error) {
	prompt := "Your previous plan had these problems:\n" + feedback +
		"\n\nRegenerate the complete corrected plan."
	raw, err := o.chat(ctx, prompt)
	if err != nil {
		return plan.Plan{}, err
	}
	obj, ok := plan.ExtractJSON(raw)
	if !ok {
		return plan.Plan{}, fmt.Errorf("the model did not return a usable plan")
	}
	return plan.Decode(obj)
}

// repairInstruction is the nudge sent when a reply could not be parsed.
const repairInstruction = `Your previous reply was not a valid JSON object of the required shape. ` +
	`Respond again with ONLY the JSON object: {"commands":[{"command":"...","risk":"..."}],` +
	`"explanation":"...","caveat":"..."}`

// chat sends the conversation plus a user message and records both the
// message and the reply, so that the next turn has the full context.
func (o *OpenAI) chat(ctx context.Context, userContent string) (string, error) {
	messages := append(append([]message{}, o.history...), message{Role: roleUser, Content: userContent})
	content, err := o.send(ctx, messages)
	if err != nil {
		return "", err
	}
	o.history = append(o.history,
		message{Role: roleUser, Content: userContent},
		message{Role: roleAssistant, Content: content})
	return content, nil
}

// chatCompletion is the request body of a chat completion.
type chatCompletion struct {
	Model          string         `json:"model"`
	Messages       []message      `json:"messages"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
}

// chatResponse is the part of the reply nlsh cares about.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content *string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// send performs one chat completion request.
//
// The plan schema is sent as response_format.json_schema, which makes a
// compliant server return a plan that always parses. Servers that do not
// support it answer 400, and the request is retried with the looser
// json_object format.
func (o *OpenAI) send(ctx context.Context, messages []message) (string, error) {
	if o.cfg.Model == "" {
		return "", &Error{Op: "chat completion", Body: "no model configured; set NLSH_MODEL or pass --model"}
	}
	schema, err := plan.SchemaObject()
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}
	body := chatCompletion{
		Model:       o.cfg.Model,
		Messages:    messages,
		Temperature: 0.2,
		ResponseFormat: map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "plan",
				"strict": true,
				"schema": schema,
			},
		},
	}

	content, err := o.post(ctx, body)
	if err == nil {
		return content, nil
	}

	// Fall back for servers that reject the schema.
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
		return "", err
	}
	body.ResponseFormat = map[string]any{"type": "json_object"}
	body.Messages = append(append([]message{}, body.Messages...), message{
		Role:    roleUser,
		Content: "Respond with a single JSON object and nothing else, with the shape {\"commands\":[{\"command\":\"...\",\"risk\":\"safe|careful|destructive\"}],\"explanation\":\"...\",\"caveat\":\"...\"}.",
	})
	return o.post(ctx, body)
}

// post performs the HTTP request and returns the assistant's content.
func (o *OpenAI) post(ctx context.Context, body chatCompletion) (string, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}

	endpoint := strings.TrimRight(o.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}

	var decoded chatResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", &Error{
			Op:     "chat completion",
			Status: resp.StatusCode,
			Body:   truncate(string(raw), 300),
			Err:    err,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := ""
		if decoded.Error != nil {
			msg = decoded.Error.Message
		}
		if msg == "" {
			msg = truncate(string(raw), 300)
		}
		return "", &Error{Op: "chat completion", Status: resp.StatusCode, Body: msg}
	}
	if len(decoded.Choices) == 0 {
		return "", &Error{Op: "chat completion", Status: resp.StatusCode, Body: "the server returned no choices"}
	}
	content := decoded.Choices[0].Message.Content
	if content == nil {
		return "", &Error{Op: "chat completion", Status: resp.StatusCode, Body: "the server returned an empty message"}
	}
	return *content, nil
}

// truncate shortens a string for an error message.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
