package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/betty/agent/tools"
)

type EventType string

const (
	EventStatus     EventType = "status"
	EventAssistant  EventType = "assistant"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
)

type Event struct {
	Type      EventType
	Message   string
	ToolName  string
	ToolInput string
	IsError   bool
	Time      time.Time
}

type EventSink func(Event)

type Agent struct {
	Client         *anthropic.Client
	Model          anthropic.Model
	Tools          []tools.ToolDefinition
	SystemPrompt   string
	MaxTokens      int64
	RequestTimeout time.Duration
}

type Session struct {
	agent        *Agent
	mu           sync.Mutex
	conversation []anthropic.MessageParam
	running      bool
}

func NewAgent(client *anthropic.Client, model anthropic.Model, toolDefinitions []tools.ToolDefinition, systemPrompt string, maxTokens int64, requestTimeout time.Duration) *Agent {
	return &Agent{
		Client:         client,
		Model:          model,
		Tools:          toolDefinitions,
		SystemPrompt:   systemPrompt,
		MaxTokens:      maxTokens,
		RequestTimeout: requestTimeout,
	}
}

func NewSession(agent *Agent) *Session {
	return &Session{agent: agent}
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversation = nil
	s.running = false
}

func (s *Session) RunTurn(ctx context.Context, userInput string, sink EventSink) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("an agent turn is already running")
	}
	s.running = true
	defer func() {
		s.running = false
		s.mu.Unlock()
	}()

	s.emit(sink, Event{Type: EventStatus, Message: "Thinking...", Time: time.Now()})
	s.conversation = append(s.conversation, anthropic.NewUserMessage(anthropic.NewTextBlock(userInput)))

	for {
		message, err := s.agent.runInference(ctx, s.conversation)
		if err != nil {
			return err
		}

		s.conversation = append(s.conversation, message.ToParam())

		toolResults := make([]anthropic.ContentBlockParamUnion, 0)
		hasToolUse := false

		for _, content := range message.Content {
			switch content.Type {
			case "text":
				if content.Text != "" {
					s.emit(sink, Event{Type: EventAssistant, Message: content.Text, Time: time.Now()})
				}
			case "tool_use":
				hasToolUse = true
				s.emit(sink, Event{
					Type:      EventToolCall,
					ToolName:  content.Name,
					ToolInput: prettifyJSON(content.Input),
					Message:   fmt.Sprintf("Running %s", content.Name),
					Time:      time.Now(),
				})
				result, event := s.agent.executeTool(ctx, content.ID, content.Name, content.Input)
				s.emit(sink, event)
				toolResults = append(toolResults, result)
			}
		}

		if !hasToolUse {
			s.emit(sink, Event{Type: EventStatus, Message: "Idle", Time: time.Now()})
			return nil
		}

		s.conversation = append(s.conversation, anthropic.NewUserMessage(toolResults...))
		s.emit(sink, Event{Type: EventStatus, Message: "Reviewing tool results...", Time: time.Now()})
	}
}

func (s *Session) emit(sink EventSink, event Event) {
	if sink != nil {
		sink(event)
	}
}

func (a *Agent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
	requestCtx := ctx
	var cancel context.CancelFunc
	if a.RequestTimeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, a.RequestTimeout)
		defer cancel()
	}

	toolParams := make([]anthropic.ToolUnionParam, 0, len(a.Tools))
	for _, tool := range a.Tools {
		toolParams = append(toolParams, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: tool.InputSchema,
			},
		})
	}

	params := anthropic.MessageNewParams{
		Model:       a.Model,
		MaxTokens:   a.MaxTokens,
		Messages:    conversation,
		Tools:       toolParams,
		Temperature: anthropic.Float(0.2),
	}
	if a.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: a.SystemPrompt}}
	}

	return a.Client.Messages.New(requestCtx, params)
}

func (a *Agent) executeTool(ctx context.Context, id, name string, input json.RawMessage) (anthropic.ContentBlockParamUnion, Event) {
	toolDef, found := a.findTool(name)
	prettyInput := prettifyJSON(input)
	if !found {
		return anthropic.NewToolResultBlock(id, "tool not found", true), Event{
			Type:      EventToolResult,
			ToolName:  name,
			ToolInput: prettyInput,
			Message:   "tool not found",
			IsError:   true,
			Time:      time.Now(),
		}
	}

	result, err := toolDef.Function(ctx, input)
	if err != nil {
		return anthropic.NewToolResultBlock(id, resultOrFallback(result, err.Error()), true), Event{
			Type:      EventToolResult,
			ToolName:  name,
			ToolInput: prettyInput,
			Message:   resultOrFallback(result, err.Error()),
			IsError:   true,
			Time:      time.Now(),
		}
	}

	return anthropic.NewToolResultBlock(id, result, false), Event{
		Type:      EventToolResult,
		ToolName:  name,
		ToolInput: prettyInput,
		Message:   result,
		Time:      time.Now(),
	}
}

func (a *Agent) findTool(name string) (tools.ToolDefinition, bool) {
	for _, tool := range a.Tools {
		if tool.Name == name {
			return tool, true
		}
	}

	return tools.ToolDefinition{}, false
}

func prettifyJSON(input json.RawMessage) string {
	if len(input) == 0 {
		return "{}"
	}

	formatted, err := json.MarshalIndent(json.RawMessage(input), "", "  ")
	if err != nil {
		return string(input)
	}

	return string(formatted)
}

func resultOrFallback(result, fallback string) string {
	if result != "" {
		return result
	}
	return fallback
}
