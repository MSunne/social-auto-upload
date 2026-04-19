package ai

import (
	"context"
	"fmt"
	"strings"

	appstate "omnidrive_cloud/internal/app"
)

func GenerateTextWithModel(ctx context.Context, app *appstate.App, modelName string, messages []ChatMessage) (*ChatResult, error) {
	if app == nil || app.Store == nil {
		return nil, fmt.Errorf("app store is required")
	}
	trimmedModelName := strings.TrimSpace(modelName)
	if trimmedModelName == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	model, err := app.Store.GetAIModelByName(ctx, trimmedModelName)
	if err != nil {
		return nil, err
	}
	if model == nil || !model.IsEnabled {
		return nil, fmt.Errorf("ai model is disabled or missing")
	}

	providers, err := newProviders(app.Config)
	if err != nil {
		return nil, err
	}
	provider, err := resolveProviderForModel(providers, model)
	if err != nil {
		return nil, err
	}
	baseURL, apiKey := ResolveModelRuntimeConfig(app.Config, model)
	chatProtocol := NormalizeChatProtocol(model.ChatProtocol)
	if chatProtocol == "" {
		chatProtocol = ChatProtocolAuto
	}
	return provider.GenerateChat(ctx, ChatRequest{
		Model:        firstNonEmpty(trimmedModelName, strings.TrimSpace(model.ModelName)),
		BaseURL:      baseURL,
		APIKey:       apiKey,
		ChatProtocol: chatProtocol,
		Messages:     messages,
	})
}

func GenerateTextWithModelPrompt(ctx context.Context, app *appstate.App, modelName string, prompt string) (*ChatResult, error) {
	return GenerateTextWithModel(ctx, app, modelName, []ChatMessage{{
		Role:    "user",
		Content: strings.TrimSpace(prompt),
	}})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
