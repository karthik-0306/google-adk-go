package loopagent_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/agent/workflowagents/loopagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

type ErrorLLM struct{}

func (f *ErrorLLM) Name() string {
	return "error-llm"
}

func (f *ErrorLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(nil, errors.New("simulated llm error"))
	}
}

func TestLoopAgent_ErrorPropagation(t *testing.T) {
	ctx := t.Context()

	agent1, _ := llmagent.New(llmagent.Config{Name: "agent1", Model: &ErrorLLM{}})
	agent2, _ := llmagent.New(llmagent.Config{Name: "agent2", Model: &FakeLLM{id: 2}})

	loopAg, _ := loopagent.New(loopagent.Config{
		MaxIterations: 2,
		AgentConfig: agent.Config{
			Name:      "loop",
			SubAgents: []agent.Agent{agent1, agent2},
		},
	})

	sessionService := session.InMemoryService()
	sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "test_app",
		UserID:    "user_id",
		SessionID: "session_id",
	})

	r, _ := runner.New(runner.Config{
		AppName:        "test",
		Agent:          loopAg,
		SessionService: sessionService,
	})

	var gotErr error
	var hasAgent2Event bool
	for ev, err := range r.Run(ctx, "user_id", "session_id", genai.NewContentFromText("test", genai.RoleUser), agent.RunConfig{}) {
		if err != nil {
			gotErr = err
		}
		if ev != nil && ev.Author == "agent2" {
			hasAgent2Event = true
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from agent1")
	}
	if hasAgent2Event {
		t.Errorf("agent2 was run despite agent1 returning an error")
	}
}
