package classify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/genai"
)

const (
	DefaultModel    = "gemini-3.6-flash"
	callTimeout     = 10 * time.Minute
	maxOutputTokens = 32768
)

type Gemini struct {
	client *genai.Client
	model  string
	schema *genai.Schema
}

func NewGemini(ctx context.Context, apiKey, model string) (*Gemini, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}
	return &Gemini{client: client, model: model, schema: Schema()}, nil
}

func (g *Gemini) Generate(ctx context.Context, system, user string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(system, genai.RoleUser),
		ResponseMIMEType:  "application/json",
		ResponseSchema:    g.schema,
		Temperature:       new(float32(0)),
		MaxOutputTokens:   maxOutputTokens,
	}
	resp, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(user), cfg)
	if err != nil {
		return "", err
	}
	if u := resp.UsageMetadata; u != nil {
		slog.Info("gemini usage", "model", g.model, "in", u.PromptTokenCount,
			"out", u.CandidatesTokenCount+u.ThoughtsTokenCount)
	}
	text := resp.Text()
	if text == "" {
		return "", fmt.Errorf("empty reply: %s", emptyReason(resp))
	}
	return text, nil
}

func emptyReason(resp *genai.GenerateContentResponse) string {
	reason := "finish_reason=none"
	if len(resp.Candidates) > 0 && resp.Candidates[0] != nil {
		reason = "finish_reason=" + string(resp.Candidates[0].FinishReason)
	}
	if pf := resp.PromptFeedback; pf != nil {
		reason += " block_reason=" + string(pf.BlockReason)
	}
	return reason
}
