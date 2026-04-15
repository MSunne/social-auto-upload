package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/digitalhuman"
	"omnidrive_cloud/internal/http/render"
)

type digitalHumanModelOption struct {
	ID            string `json:"id"`
	IsCurrent     bool   `json:"isCurrent"`
	IsRecommended bool   `json:"isRecommended"`
}

type digitalHumanModeDefaults struct {
	Digital   string `json:"digital"`
	Customize string `json:"customize"`
}

type digitalHumanModelsResponse struct {
	RecommendedModelID string                    `json:"recommendedModelId"`
	CurrentModelID     string                    `json:"currentModelId"`
	Provider           string                    `json:"provider,omitempty"`
	DefaultModelByMode digitalHumanModeDefaults  `json:"defaultModelByMode"`
	Models             []digitalHumanModelOption `json:"models"`
}

func loadDigitalHumanModelsResponse(ctx context.Context, app *appstate.App) (*digitalHumanModelsResponse, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}

	settings, err := loadEffectiveAdminSystemSettings(ctx, app)
	if err != nil {
		return nil, err
	}

	client := digitalhuman.NewClient(app.Config)
	upstream, _, err := client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	models := make([]digitalHumanModelOption, 0, len(upstream.Models))
	seen := make(map[string]struct{}, len(upstream.Models))
	for _, item := range upstream.Models {
		modelID := strings.TrimSpace(item.ID)
		if modelID == "" {
			continue
		}
		if _, exists := seen[modelID]; exists {
			continue
		}
		seen[modelID] = struct{}{}
		models = append(models, digitalHumanModelOption{
			ID:            modelID,
			IsCurrent:     item.IsCurrent,
			IsRecommended: false,
		})
	}

	currentModelID := strings.TrimSpace(upstream.CurrentModel)
	defaults := digitalHumanModeDefaults{
		Digital: resolveDigitalHumanDefaultModel(
			strings.TrimSpace(settings.DigitalHumanShoppingDefaultModel),
			currentModelID,
			models,
		),
		Customize: resolveDigitalHumanDefaultModel(
			strings.TrimSpace(settings.DigitalHumanSpeechDefaultModel),
			currentModelID,
			models,
		),
	}

	return &digitalHumanModelsResponse{
		RecommendedModelID: "",
		CurrentModelID:     currentModelID,
		Provider:           strings.TrimSpace(upstream.Provider),
		DefaultModelByMode: defaults,
		Models:             models,
	}, nil
}

func resolveDigitalHumanDefaultModel(preferred string, currentModelID string, models []digitalHumanModelOption) string {
	if containsDigitalHumanModel(models, preferred) {
		return preferred
	}
	if containsDigitalHumanModel(models, currentModelID) {
		return currentModelID
	}
	if len(models) > 0 {
		return models[0].ID
	}
	return firstNonEmptyDigitalHumanString(preferred, currentModelID)
}

func containsDigitalHumanModel(models []digitalHumanModelOption, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	for _, item := range models {
		if item.ID == modelName {
			return true
		}
	}
	return false
}

func resolveDigitalHumanDefaultModelByMode(config *digitalHumanModelsResponse, mode string) string {
	if config == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "digital":
		return resolveDigitalHumanDefaultModel(config.DefaultModelByMode.Digital, config.CurrentModelID, config.Models)
	case "customize":
		return resolveDigitalHumanDefaultModel(config.DefaultModelByMode.Customize, config.CurrentModelID, config.Models)
	default:
		return resolveDigitalHumanDefaultModel("", config.CurrentModelID, config.Models)
	}
}

func validateDigitalHumanModelName(ctx context.Context, app *appstate.App, modelName string) error {
	config, err := loadDigitalHumanModelsResponse(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to load digital human models: %w", err)
	}
	trimmed := strings.TrimSpace(modelName)
	if trimmed == "" {
		return fmt.Errorf("must not be empty")
	}
	if !containsDigitalHumanModel(config.Models, trimmed) {
		return fmt.Errorf("must reference an available digital human model")
	}
	return nil
}

func renderDigitalHumanModels(w http.ResponseWriter, r *http.Request, app *appstate.App) {
	config, err := loadDigitalHumanModelsResponse(r.Context(), app)
	if err != nil {
		render.Error(w, http.StatusBadGateway, "Failed to load digital human models")
		return
	}
	render.JSON(w, http.StatusOK, config)
}

func firstNonEmptyDigitalHumanString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
