package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	baseURL      string
	email        string
	password     string
	token        string
	mode         string
	pollInterval time.Duration
	timeout      time.Duration
	chatPrompt   string
	imagePrompt  string
	videoPrompt  string
	imageRefs    []string
	videoRefs    []string
	videoRatio   string
	videoSeconds int
}

type authResponse struct {
	AccessToken string `json:"accessToken"`
}

type aiJob struct {
	ID          string          `json:"id"`
	JobType     string          `json:"jobType"`
	ModelName   string          `json:"modelName"`
	Status      string          `json:"status"`
	Message     *string         `json:"message"`
	CostCredits int64           `json:"costCredits"`
	Prompt      *string         `json:"prompt"`
	Output      json.RawMessage `json:"outputPayload"`
}

type aiArtifact struct {
	ArtifactKey  string  `json:"artifactKey"`
	ArtifactType string  `json:"artifactType"`
	FileName     *string `json:"fileName"`
	MimeType     *string `json:"mimeType"`
	PublicURL    *string `json:"publicUrl"`
}

type aiWorkspace struct {
	Job       aiJob        `json:"job"`
	Artifacts []aiArtifact `json:"artifacts"`
}

func main() {
	cfg := loadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	client := &http.Client{Timeout: 2 * time.Minute}
	token := strings.TrimSpace(cfg.token)
	if token == "" {
		var err error
		token, err = login(ctx, client, cfg)
		if err != nil {
			fatalf("login failed: %v", err)
		}
	}

	modes := selectedModes(cfg.mode)
	failed := false
	for _, mode := range modes {
		if err := runScenario(ctx, client, cfg, token, mode); err != nil {
			failed = true
			fmt.Fprintf(os.Stderr, "[%s] failed: %v\n", mode, err)
		}
	}
	if failed {
		os.Exit(1)
	}
}

func loadConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.baseURL, "base-url", envOrDefault("OMNIDRIVE_SMOKE_BASE_URL", "http://127.0.0.1:8410"), "OmniDrive API base URL")
	flag.StringVar(&cfg.email, "email", envOrDefault("OMNIDRIVE_SMOKE_EMAIL", ""), "OmniDrive login email")
	flag.StringVar(&cfg.password, "password", envOrDefault("OMNIDRIVE_SMOKE_PASSWORD", ""), "OmniDrive login password")
	flag.StringVar(&cfg.token, "token", envOrDefault("OMNIDRIVE_SMOKE_TOKEN", ""), "Existing bearer token")
	flag.StringVar(&cfg.mode, "mode", envOrDefault("OMNIDRIVE_SMOKE_MODE", "all"), "chat|image|video|all")
	flag.DurationVar(&cfg.pollInterval, "poll-interval", envDurationOrDefault("OMNIDRIVE_SMOKE_POLL_INTERVAL", 5*time.Second), "Polling interval")
	flag.DurationVar(&cfg.timeout, "timeout", envDurationOrDefault("OMNIDRIVE_SMOKE_TIMEOUT", 15*time.Minute), "Overall timeout")
	flag.StringVar(&cfg.chatPrompt, "chat-prompt", envOrDefault("OMNIDRIVE_SMOKE_CHAT_PROMPT", "请给我 3 条适合短视频产品宣传的中文创意标题。"), "Chat prompt")
	flag.StringVar(&cfg.imagePrompt, "image-prompt", envOrDefault("OMNIDRIVE_SMOKE_IMAGE_PROMPT", "生成一张适合电商海报的干净产品图，主体突出，背景简洁。"), "Image prompt")
	flag.StringVar(&cfg.videoPrompt, "video-prompt", envOrDefault("OMNIDRIVE_SMOKE_VIDEO_PROMPT", "生成一个节奏轻快的产品展示短视频，镜头平稳，适合社媒传播。"), "Video prompt")
	flag.StringVar(&cfg.videoRatio, "video-ratio", envOrDefault("OMNIDRIVE_SMOKE_VIDEO_RATIO", "9:16"), "Video aspect ratio")
	flag.IntVar(&cfg.videoSeconds, "video-seconds", envIntOrDefault("OMNIDRIVE_SMOKE_VIDEO_SECONDS", 8), "Video duration seconds")
	flag.Parse()

	cfg.baseURL = strings.TrimRight(strings.TrimSpace(cfg.baseURL), "/")
	cfg.email = strings.TrimSpace(cfg.email)
	cfg.password = strings.TrimSpace(cfg.password)
	cfg.token = strings.TrimSpace(cfg.token)
	cfg.mode = strings.TrimSpace(strings.ToLower(cfg.mode))
	cfg.imageRefs = splitCSV(envOrDefault("OMNIDRIVE_SMOKE_IMAGE_REFS", ""))
	cfg.videoRefs = splitCSV(envOrDefault("OMNIDRIVE_SMOKE_VIDEO_REFS", ""))
	return cfg
}

func selectedModes(mode string) []string {
	switch mode {
	case "", "all":
		return []string{"chat", "image", "video"}
	case "chat", "image", "video":
		return []string{mode}
	default:
		fatalf("unsupported mode %q", mode)
		return nil
	}
}

func login(ctx context.Context, client *http.Client, cfg config) (string, error) {
	if cfg.email == "" || cfg.password == "" {
		return "", errors.New("email/password or token is required")
	}
	var response authResponse
	err := doJSON(ctx, client, http.MethodPost, cfg.baseURL+"/api/v1/auth/login", "", map[string]any{
		"email":    cfg.email,
		"password": cfg.password,
	}, &response)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", errors.New("login response did not include accessToken")
	}
	return response.AccessToken, nil
}

func runScenario(ctx context.Context, client *http.Client, cfg config, token string, mode string) error {
	request := map[string]any{}
	switch mode {
	case "chat":
		request = map[string]any{
			"jobType":   "chat",
			"modelName": "gemini-3.1-pro-preview",
			"prompt":    cfg.chatPrompt,
		}
	case "image":
		request = map[string]any{
			"jobType":   "image",
			"modelName": "gemini-3-pro-image-preview",
			"prompt":    cfg.imagePrompt,
			"inputPayload": map[string]any{
				"referenceImages": cfg.imageRefs,
				"aspectRatio":     "9:16",
			},
		}
	case "video":
		request = map[string]any{
			"jobType":   "video",
			"modelName": "veo-3.1-fast-fl",
			"prompt":    cfg.videoPrompt,
			"inputPayload": map[string]any{
				"referenceImages": cfg.videoRefs,
				"aspectRatio":     cfg.videoRatio,
				"durationSeconds": cfg.videoSeconds,
			},
		}
	default:
		return fmt.Errorf("unsupported mode %s", mode)
	}

	var created aiJob
	if err := doJSON(ctx, client, http.MethodPost, cfg.baseURL+"/api/v1/ai/jobs", token, request, &created); err != nil {
		return err
	}
	fmt.Printf("[%s] created job %s model=%s status=%s\n", mode, created.ID, created.ModelName, created.Status)

	job, err := waitForJob(ctx, client, cfg, token, created.ID)
	if err != nil {
		return err
	}

	var workspace aiWorkspace
	if err := doJSON(ctx, client, http.MethodGet, cfg.baseURL+"/api/v1/ai/jobs/"+job.ID+"/workspace", token, nil, &workspace); err != nil {
		return err
	}
	printWorkspace(mode, workspace)
	if workspace.Job.Status != "success" && workspace.Job.Status != "completed" {
		return fmt.Errorf("job ended with status=%s message=%s", workspace.Job.Status, stringValue(workspace.Job.Message))
	}
	return nil
}

func waitForJob(ctx context.Context, client *http.Client, cfg config, token string, jobID string) (*aiJob, error) {
	ticker := time.NewTicker(cfg.pollInterval)
	defer ticker.Stop()

	for {
		var job aiJob
		if err := doJSON(ctx, client, http.MethodGet, cfg.baseURL+"/api/v1/ai/jobs/"+jobID, token, nil, &job); err != nil {
			return nil, err
		}
		fmt.Printf("  poll job=%s status=%s cost=%d message=%s\n", job.ID, job.Status, job.CostCredits, stringValue(job.Message))
		switch job.Status {
		case "success", "completed", "failed", "cancelled":
			return &job, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func printWorkspace(mode string, workspace aiWorkspace) {
	fmt.Printf("[%s] final status=%s cost=%d artifacts=%d\n", mode, workspace.Job.Status, workspace.Job.CostCredits, len(workspace.Artifacts))
	for _, artifact := range workspace.Artifacts {
		fmt.Printf("  - key=%s type=%s file=%s mime=%s url=%s\n",
			artifact.ArtifactKey,
			artifact.ArtifactType,
			stringValue(artifact.FileName),
			stringValue(artifact.MimeType),
			stringValue(artifact.PublicURL),
		)
	}
	if len(workspace.Job.Output) > 0 {
		pretty := prettyJSON(workspace.Job.Output)
		if len(pretty) > 1200 {
			pretty = pretty[:1200] + "..."
		}
		fmt.Printf("  output=%s\n", pretty)
	}
}

func doJSON(ctx context.Context, client *http.Client, method string, rawURL string, token string, payload any, destination any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &apiError) == nil && strings.TrimSpace(apiError.Error) != "" {
			return fmt.Errorf("%s %s failed: %s", method, rawURL, apiError.Error)
		}
		return fmt.Errorf("%s %s failed: status=%d body=%s", method, rawURL, resp.StatusCode, string(data))
	}
	if destination == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, destination)
}

func prettyJSON(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return string(data)
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(pretty)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return result
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
