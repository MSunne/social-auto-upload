package handlers

import (
	"encoding/json"
	"strings"

	"omnidrive_cloud/internal/domain"
)

type digitalHumanSubmissionRequest struct {
	CharacterAssetPath string  `json:"character_asset_path"`
	Mode               string  `json:"mode"`
	GoodsAssetPath     *string `json:"goods_asset_path,omitempty"`
	GoodsText          string  `json:"goods_text"`
	GoodsTitle         *string `json:"goods_title,omitempty"`
}

type digitalHumanRequestAuditPayload struct {
	SubmissionRequest json.RawMessage `json:"submissionRequest"`
}

func decorateDigitalHumanTaskForResponse(task *domain.DigitalHumanTask) {
	if task == nil {
		return
	}
	task.SubmissionEvidence = buildDigitalHumanSubmissionEvidence(task)
}

func decorateDigitalHumanTaskSliceForResponse(tasks []domain.DigitalHumanTask) {
	for index := range tasks {
		decorateDigitalHumanTaskForResponse(&tasks[index])
	}
}

func buildDigitalHumanSubmissionEvidence(task *domain.DigitalHumanTask) *domain.DigitalHumanSubmissionEvidence {
	if task == nil {
		return nil
	}
	request, ok := extractDigitalHumanSubmissionRequest(task.RequestPayload)
	if !ok {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if mode == "" {
		return nil
	}
	evidence := &domain.DigitalHumanSubmissionEvidence{
		Mode:         mode,
		RemoteTaskID: task.RemoteTaskID,
	}
	if mode == "customize" {
		return evidence
	}
	if request.GoodsTitle != nil {
		if trimmed := strings.TrimSpace(*request.GoodsTitle); trimmed != "" {
			evidence.GoodsTitle = &trimmed
		}
	}
	if request.GoodsAssetPath != nil && strings.TrimSpace(*request.GoodsAssetPath) != "" {
		evidence.HasGoodsAssetPath = true
	}
	return evidence
}

func extractDigitalHumanSubmissionRequest(raw json.RawMessage) (*digitalHumanSubmissionRequest, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	submissionRaw := raw
	var auditPayload digitalHumanRequestAuditPayload
	if err := json.Unmarshal(raw, &auditPayload); err == nil && len(auditPayload.SubmissionRequest) > 0 {
		submissionRaw = auditPayload.SubmissionRequest
	}

	var request digitalHumanSubmissionRequest
	if err := json.Unmarshal(submissionRaw, &request); err != nil {
		return nil, false
	}
	if strings.TrimSpace(request.CharacterAssetPath) == "" || strings.TrimSpace(request.GoodsText) == "" {
		return nil, false
	}
	request.Mode = strings.ToLower(strings.TrimSpace(request.Mode))
	if request.Mode == "" {
		return nil, false
	}
	if request.Mode == "customize" {
		request.GoodsTitle = nil
		request.GoodsAssetPath = nil
		return &request, true
	}
	if request.GoodsTitle != nil {
		if trimmed := strings.TrimSpace(*request.GoodsTitle); trimmed != "" {
			request.GoodsTitle = &trimmed
		} else {
			request.GoodsTitle = nil
		}
	}
	if request.GoodsAssetPath != nil {
		if trimmed := strings.TrimSpace(*request.GoodsAssetPath); trimmed != "" {
			request.GoodsAssetPath = &trimmed
		} else {
			request.GoodsAssetPath = nil
		}
	}
	return &request, true
}
