package plan

import "encoding/json"

// Schema is the JSON Schema nlsh sends as response_format.json_schema.
//
// Sending the schema (rather than describing the shape in prose and hoping)
// means a compliant backend returns a plan that always decodes. Backends
// that do not implement json_schema are retried with json_object and the
// old "respond with only JSON" instruction; see internal/backend.
//
// Keep this in sync with the Plan and Command types above.
const Schema = `{
  "type": "object",
  "properties": {
    "commands": {
      "type": "array",
      "description": "Ordered shell commands that fulfil the request. Empty if the request cannot be fulfilled unattended.",
      "items": {
        "type": "object",
        "properties": {
          "command": {
            "type": "string",
            "description": "One complete shell command line, ready to run unattended. No sudo, no interactive programs, no comments."
          },
          "risk": {
            "type": "string",
            "enum": ["safe", "careful", "destructive"],
            "description": "safe = read-only. careful = creates or changes files or state. destructive = deletes, overwrites, resets or force-pushes."
          }
        },
        "required": ["command", "risk"]
      }
    },
    "explanation": {
      "type": "string",
      "description": "One short sentence, written to the user, describing what the commands will do."
    },
    "caveat": {
      "type": "string",
      "description": "Brief note on anything assumed, uncertain or surprising. Empty string if there is nothing to say."
    }
  },
  "required": ["commands", "explanation", "caveat"]
}`

// SchemaObject returns Schema decoded into a generic map, ready to be
// embedded in a chat-completions request.
func SchemaObject() (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(Schema), &m); err != nil {
		return nil, err
	}
	return m, nil
}
