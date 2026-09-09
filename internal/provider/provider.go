package provider

import "context"

type Input struct {
	Type string `json:"type"`
	Role string `json:"role"`
	URL  string `json:"url"`
}

type Request struct {
	TaskType string         `json:"taskType"`
	Model    string         `json:"model"`
	Prompt   string         `json:"prompt"`
	Inputs   []Input        `json:"inputs,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

type Result struct {
	Done       bool
	Failed     bool
	ExternalID string
	URL        string
	Data       []byte
	MIME       string
	Raw        []byte
	Error      string
}

type Provider interface {
	Submit(ctx context.Context, req Request) (Result, error)
	Poll(ctx context.Context, externalID string) (Result, error)
}

type Model struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	TaskType    string         `json:"taskType"`
	Description string         `json:"description"`
	Inputs      []string       `json:"inputs"`
	Defaults    map[string]any `json:"defaults"`
	Options     map[string]any `json:"options"`
}
