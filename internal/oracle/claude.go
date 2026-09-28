package oracle

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// The model the gods speak through
const Model = "claude-opus-5"

// Answers prayers with Claude. The API key comes from ANTHROPIC_API_KEY.
type Claude struct {
	client anthropic.Client
	// Picks which god answers, for tests
	pick func(n int) int
}

func NewClaude(opts ...option.RequestOption) *Claude {
	return &Claude{client: anthropic.NewClient(opts...), pick: rand.IntN}
}

// A random god answers. An answer that gives the word away is asked for
// again once.
func (c *Claude) Pray(ctx context.Context, p Prayer) (Answer, error) {
	god := Gods[c.pick(len(Gods))]
	for range 2 {
		text, err := c.ask(ctx, god, p)
		if err != nil {
			return Answer{}, err
		}
		if !leaks(p, text) {
			return Answer{God: god, Text: text}, nil
		}
	}
	return Answer{}, fmt.Errorf("%w: every answer gave the word away", ErrUnanswered)
}

func (c *Claude) ask(ctx context.Context, god God, p Prayer) (string, error) {
	resp, err := c.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     Model,
		MaxTokens: 2000,
		System:    []anthropic.BetaTextBlockParam{{Text: systemPrompt(god, len(p.Word))}},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(userPrompt(p))),
		},
		// A riddle doesn't need deep thought, and the player is waiting
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortLow},
		// If Claude declines for policy reasons, a fallback model answers instead
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
	})
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnanswered, err)
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return "", fmt.Errorf("%w: declined", ErrUnanswered)
	}
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "", fmt.Errorf("%w: empty answer", ErrUnanswered)
	}
	return text, nil
}
