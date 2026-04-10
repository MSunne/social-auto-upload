package digitalhuman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"omnidrive_cloud/internal/config"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type GenerateRequest struct {
	CharacterAssetPath string  `json:"character_asset_path"`
	Mode               string  `json:"mode"`
	GoodsAssetPath     *string `json:"goods_asset_path,omitempty"`
	GoodsText          string  `json:"goods_text"`
	GoodsTitle         *string `json:"goods_title,omitempty"`
	Source             string  `json:"source"`
	RefAudio           string  `json:"ref_audio"`
}

type GenerateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TaskID  string `json:"task_id"`
}

type TaskProgress struct {
	Current    int     `json:"current"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
	Message    string  `json:"message"`
}

type RemoteTask struct {
	TaskID        string         `json:"task_id"`
	TaskType      string         `json:"task_type"`
	Status        string         `json:"status"`
	Progress      *TaskProgress  `json:"progress"`
	Result        map[string]any `json:"result"`
	Error         *string        `json:"error"`
	RequestParams map[string]any `json:"request_params"`
}

func NewClient(cfg config.Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.DigitalHumanBaseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) GenerateVideo(ctx context.Context, req GenerateRequest) (*GenerateResponse, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	respBody, err := c.doJSON(ctx, http.MethodPost, "/api/digital-human-flow/step3-generate-video", body)
	if err != nil {
		return nil, nil, err
	}

	var resp GenerateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, respBody, fmt.Errorf("decode digital human generate response: %w", err)
	}
	if strings.TrimSpace(resp.TaskID) == "" {
		return nil, respBody, fmt.Errorf("digital human generate response missing task_id")
	}
	return &resp, respBody, nil
}

func (c *Client) GetTask(ctx context.Context, taskID string) (*RemoteTask, []byte, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, nil, fmt.Errorf("task id is required")
	}
	respBody, err := c.doJSON(ctx, http.MethodGet, "/api/digital-human-flow/step4-check-status/"+url.PathEscape(strings.TrimSpace(taskID)), nil)
	if err != nil {
		return nil, nil, err
	}

	var task RemoteTask
	if err := json.Unmarshal(respBody, &task); err != nil {
		return nil, respBody, fmt.Errorf("decode digital human task response: %w", err)
	}
	if strings.TrimSpace(task.TaskID) == "" {
		task.TaskID = strings.TrimSpace(taskID)
	}
	return &task, respBody, nil
}

func (c *Client) doJSON(ctx context.Context, method string, endpoint string, body []byte) ([]byte, error) {
	targetURL := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respBody bytes.Buffer
	if _, err := respBody.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("digital human api %s %s returned %d: %s", method, endpoint, resp.StatusCode, strings.TrimSpace(respBody.String()))
	}
	return respBody.Bytes(), nil
}
