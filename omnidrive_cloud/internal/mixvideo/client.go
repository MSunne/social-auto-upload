package mixvideo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omnidrive_cloud/internal/config"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type GenerateRequest struct {
	AssetPaths   []string
	RefAudioPath string
	ScriptText   string
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
	TaskID    string         `json:"task_id"`
	TaskType  string         `json:"task_type"`
	Status    string         `json:"status"`
	Progress  *TaskProgress  `json:"progress"`
	Result    any            `json:"result"`
	Error     *string        `json:"error"`
	CreatedAt *string        `json:"created_at"`
	UpdatedAt *string        `json:"updated_at"`
	Payload   map[string]any `json:"payload"`
}

func NewClient(cfg config.Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.MixVideoBaseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) GenerateAsync(ctx context.Context, req GenerateRequest) (*GenerateResponse, []byte, error) {
	if strings.TrimSpace(req.RefAudioPath) == "" {
		return nil, nil, fmt.Errorf("ref audio path is required")
	}
	if strings.TrimSpace(req.ScriptText) == "" {
		return nil, nil, fmt.Errorf("script text is required")
	}
	if len(req.AssetPaths) == 0 {
		return nil, nil, fmt.Errorf("at least one asset is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, path := range req.AssetPaths {
		if err := writeMultipartFile(writer, "assets", path); err != nil {
			return nil, nil, err
		}
	}
	if err := writeMultipartFile(writer, "ref_audio", req.RefAudioPath); err != nil {
		return nil, nil, err
	}
	if err := writer.WriteField("script_text", strings.TrimSpace(req.ScriptText)); err != nil {
		return nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	respBody, err := c.do(ctx, http.MethodPost, "/api/custom-script-assets/generate/async", writer.FormDataContentType(), &body)
	if err != nil {
		return nil, nil, err
	}
	var resp GenerateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, respBody, fmt.Errorf("decode mix video generate response: %w", err)
	}
	if strings.TrimSpace(resp.TaskID) == "" {
		return nil, respBody, fmt.Errorf("mix video generate response missing task_id")
	}
	return &resp, respBody, nil
}

func (c *Client) GetTask(ctx context.Context, taskID string) (*RemoteTask, []byte, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil, fmt.Errorf("task id is required")
	}
	respBody, err := c.do(ctx, http.MethodGet, "/api/custom-script-assets/tasks/"+url.PathEscape(taskID), "", nil)
	if err != nil {
		return nil, nil, err
	}

	var task RemoteTask
	if err := json.Unmarshal(respBody, &task); err != nil {
		return nil, respBody, fmt.Errorf("decode mix video task response: %w", err)
	}
	if strings.TrimSpace(task.TaskID) == "" {
		task.TaskID = taskID
	}
	return &task, respBody, nil
}

func (c *Client) ResolveURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", fmt.Errorf("mix video result url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse mix video result url: %w", err)
	}
	if parsed.Scheme != "" {
		if parsed.Host == "" {
			return "", fmt.Errorf("mix video result url host is required")
		}
		return parsed.String(), nil
	}
	baseParsed, err := url.Parse(strings.TrimSpace(c.baseURL))
	if err != nil {
		return "", fmt.Errorf("parse mix video base url: %w", err)
	}
	if baseParsed.Scheme == "" || baseParsed.Host == "" {
		return "", fmt.Errorf("mix video base url must include scheme and host")
	}
	return baseParsed.ResolveReference(parsed).String(), nil
}

func writeMultipartFile(writer *multipart.Writer, fieldName string, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("%s path is required", fieldName)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", fieldName, err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile(fieldName, filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy %s: %w", fieldName, err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method string, endpoint string, contentType string, body io.Reader) ([]byte, error) {
	targetURL := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respBody bytes.Buffer
	if _, err := io.Copy(&respBody, resp.Body); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mix video api %s %s returned %d: %s", method, endpoint, resp.StatusCode, strings.TrimSpace(respBody.String()))
	}
	return respBody.Bytes(), nil
}
