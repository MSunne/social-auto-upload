package digitalhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

const digitalHumanLeaseTTL = 90 * time.Second

type Worker struct {
	app           *appstate.App
	client        *Client
	pollInterval  time.Duration
	concurrency   int
	sem           chan struct{}
	activeTasks   sync.Map
	downloadVideo func(ctx context.Context, rawURL string, targetPath string) error
	probeDuration func(ctx context.Context, path string) (float64, error)
}

type resultSaveOutcome struct {
	asset                 *domain.DigitalHumanAsset
	actualDurationSeconds *int
	probeErr              error
}

func NewWorker(app *appstate.App) (*Worker, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}
	concurrency := app.Config.AIWorkerConcurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	pollSeconds := app.Config.DigitalHumanPollSeconds
	if pollSeconds <= 0 {
		pollSeconds = 5
	}
	return &Worker{
		app:          app,
		client:       NewClient(app.Config),
		pollInterval: time.Duration(pollSeconds) * time.Second,
		concurrency:  concurrency,
		sem:          make(chan struct{}, concurrency),
	}, nil
}

func (w *Worker) Start(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(1)

	w.app.Logger.Info("digital human worker started",
		"poll_interval", w.pollInterval.String(),
		"concurrency", w.concurrency,
		"base_url", w.client.baseURL,
	)

	go func() {
		defer wg.Done()
		w.run(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
		w.app.Logger.Info("digital human worker stopped")
	}
}

func (w *Worker) run(ctx context.Context) {
	w.runOnce(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	limit := w.concurrency * 4
	if limit < 8 {
		limit = 8
	}
	tasks, err := w.app.Store.ListExecutableDigitalHumanTasks(ctx, limit)
	if err != nil {
		w.app.Logger.Error("digital human worker failed to list executable tasks", "error", err)
		return
	}

	for _, task := range tasks {
		if _, loaded := w.activeTasks.LoadOrStore(task.ID, struct{}{}); loaded {
			continue
		}
		select {
		case <-ctx.Done():
			w.activeTasks.Delete(task.ID)
			return
		case w.sem <- struct{}{}:
		}

		go func(task domain.DigitalHumanTask) {
			defer func() {
				<-w.sem
				w.activeTasks.Delete(task.ID)
			}()
			w.processTask(ctx, task)
		}(task)
	}
}

func (w *Worker) processTask(ctx context.Context, task domain.DigitalHumanTask) {
	leaseToken := uuid.NewString()
	leaseExpiresAt := time.Now().UTC().Add(digitalHumanLeaseTTL)

	claimed, err := w.app.Store.ClaimDigitalHumanTaskLease(ctx, task.ID, leaseToken, leaseExpiresAt)
	if err != nil {
		w.app.Logger.Error("digital human worker failed to claim task lease", "task_id", task.ID, "error", err)
		return
	}
	if claimed == nil {
		return
	}

	stopLeaseHeartbeat := w.startLeaseHeartbeat(ctx, claimed.ID, leaseToken)
	defer stopLeaseHeartbeat()

	if strings.EqualFold(strings.TrimSpace(claimed.Status), "queued") {
		claimed, err = w.submitTask(ctx, claimed, leaseToken)
		if err != nil {
			w.app.Logger.Error("digital human worker failed to submit task", "task_id", task.ID, "error", err)
			return
		}
	}

	if claimed == nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(claimed.Status), "completed") &&
		strings.EqualFold(strings.TrimSpace(claimed.BillingStatus), "settlement_pending") {
		w.resumeSettlement(ctx, claimed)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(claimed.Status), "running") {
		return
	}
	if strings.TrimSpace(valueOrEmptyString(claimed.RemoteTaskID)) == "" {
		w.failTask(ctx, claimed, leaseToken, "数字人任务缺少远端 task_id", nil, nil)
		return
	}

	for {
		if ctx.Err() != nil {
			return
		}
		if _, err := w.app.Store.RenewDigitalHumanTaskLease(ctx, claimed.ID, leaseToken, time.Now().UTC().Add(digitalHumanLeaseTTL)); err != nil {
			w.app.Logger.Error("digital human worker failed to renew task lease", "task_id", claimed.ID, "error", err)
			return
		}

		remoteTask, rawResponse, err := w.client.GetTask(ctx, valueOrEmptyString(claimed.RemoteTaskID))
		if err != nil {
			w.failTask(ctx, claimed, leaseToken, buildFailureMessage(err), claimed.WorkingDir, nil)
			w.app.Logger.Error("digital human worker failed to poll remote task", "task_id", claimed.ID, "error", err)
			return
		}

		progressPayload := buildProgressPayload(remoteTask)
		progressJSON := mustJSON(progressPayload)

		switch normalizeRemoteStatus(remoteTask.Status) {
		case "completed":
			w.completeTask(ctx, claimed, leaseToken, remoteTask, rawResponse, progressJSON)
			return
		case "failed":
			w.failTask(ctx, claimed, leaseToken, firstNonEmptyString(valueOrEmptyString(remoteTask.Error), "数字人视频生成失败"), claimed.WorkingDir, rawResponse)
			return
		case "cancelled":
			w.cancelTask(ctx, claimed, leaseToken, progressJSON, rawResponse)
			return
		default:
			updated, syncErr := w.app.Store.SyncDigitalHumanTaskExecution(ctx, claimed.ID, leaseToken, store.UpdateDigitalHumanTaskExecutionInput{
				Status:                stringPtr("running"),
				Progress:              progressJSON,
				ProgressTouched:       true,
				RemoteResponsePayload: rawResponse,
				RemotePayloadTouched:  true,
			})
			if syncErr != nil {
				w.app.Logger.Error("digital human worker failed to sync running task", "task_id", claimed.ID, "error", syncErr)
				return
			}
			if updated != nil {
				claimed = updated
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(w.pollInterval):
		}
	}
}

func (w *Worker) completeTask(
	ctx context.Context,
	task *domain.DigitalHumanTask,
	leaseToken string,
	remoteTask *RemoteTask,
	rawResponse []byte,
	progressJSON []byte,
) {
	outcome, err := w.saveResultAsset(ctx, task, remoteTask)
	if err != nil {
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), task.WorkingDir, rawResponse)
		w.app.Logger.Error("digital human worker failed to mirror result", "task_id", task.ID, "error", err)
		return
	}
	if task.WorkingDir != nil {
		cleanupWorkingDir(w.app.Logger, *task.WorkingDir)
	}

	input := store.UpdateDigitalHumanTaskExecutionInput{
		Status:                stringPtr("completed"),
		ResultAsset:           mustJSON(outcome.asset),
		ResultAssetTouched:    true,
		Progress:              progressJSON,
		ProgressTouched:       true,
		RemoteResponsePayload: rawResponse,
		RemotePayloadTouched:  true,
		CompletedAt:           timePtr(time.Now().UTC()),
		CompletedTouched:      true,
		WorkingDir:            nil,
		WorkingDirTouched:     true,
		BillingStatus:         stringPtr("settlement_pending"),
		BillingStatusTouched:  true,
	}
	if outcome.actualDurationSeconds != nil {
		input.ActualDurationSeconds = outcome.actualDurationSeconds
		input.ActualDurationTouched = true
	}

	updated, syncErr := w.app.Store.SyncDigitalHumanTaskExecution(ctx, task.ID, leaseToken, input)
	if syncErr != nil {
		w.app.Logger.Error("digital human worker failed to mark task complete", "task_id", task.ID, "error", syncErr)
		return
	}
	if updated != nil {
		task = updated
	}

	if outcome.probeErr != nil {
		message := fmt.Sprintf("成品已生成，但读取视频时长失败：%s，系统稍后会自动重试结算", buildFailureMessage(outcome.probeErr))
		if _, markErr := w.app.Store.MarkDigitalHumanTaskSettlementPending(ctx, task.ID, nil, nil, message); markErr != nil {
			w.app.Logger.Error("digital human worker failed to mark settlement pending after probe failure", "task_id", task.ID, "error", markErr)
		}
		return
	}
	if outcome.actualDurationSeconds == nil {
		if _, markErr := w.app.Store.MarkDigitalHumanTaskSettlementPending(ctx, task.ID, nil, nil, "成品已生成，但缺少实际时长，系统稍后会自动重试结算"); markErr != nil {
			w.app.Logger.Error("digital human worker failed to mark settlement pending without duration", "task_id", task.ID, "error", markErr)
		}
		return
	}

	w.settleCompletedTask(ctx, task, *outcome.actualDurationSeconds)
}

func (w *Worker) cancelTask(
	ctx context.Context,
	task *domain.DigitalHumanTask,
	leaseToken string,
	progressJSON []byte,
	rawResponse []byte,
) {
	if task.WorkingDir != nil {
		cleanupWorkingDir(w.app.Logger, *task.WorkingDir)
	}
	updated, err := w.app.Store.SyncDigitalHumanTaskExecution(ctx, task.ID, leaseToken, store.UpdateDigitalHumanTaskExecutionInput{
		Status:                stringPtr("cancelled"),
		Progress:              progressJSON,
		ProgressTouched:       true,
		RemoteResponsePayload: rawResponse,
		RemotePayloadTouched:  true,
		ErrorMessage:          stringPtr("数字人任务已取消"),
		CompletedAt:           timePtr(time.Now().UTC()),
		CompletedTouched:      true,
		WorkingDir:            nil,
		WorkingDirTouched:     true,
	})
	if err != nil {
		w.app.Logger.Error("digital human worker failed to mark task cancelled", "task_id", task.ID, "error", err)
		return
	}
	if updated != nil {
		task = updated
	}
	if _, refundErr := w.app.Store.RefundDigitalHumanTaskOnFailure(ctx, task.ID, "数字人任务已取消，已退回预扣积分"); refundErr != nil {
		w.app.Logger.Error("digital human worker failed to refund cancelled task", "task_id", task.ID, "error", refundErr)
	}
}

func (w *Worker) resumeSettlement(ctx context.Context, task *domain.DigitalHumanTask) {
	actualDurationSeconds := task.ActualDurationSeconds
	if actualDurationSeconds == nil {
		probedDuration, err := w.probeStoredResultDuration(ctx, task)
		if err != nil {
			message := fmt.Sprintf("成品已生成，但读取视频时长失败：%s，系统稍后会自动重试结算", buildFailureMessage(err))
			if _, markErr := w.app.Store.MarkDigitalHumanTaskSettlementPending(ctx, task.ID, nil, nil, message); markErr != nil {
				w.app.Logger.Error("digital human worker failed to refresh settlement pending state", "task_id", task.ID, "error", markErr)
			}
			return
		}
		actualDurationSeconds = probedDuration
	}

	w.settleCompletedTask(ctx, task, *actualDurationSeconds)
}

func (w *Worker) settleCompletedTask(ctx context.Context, task *domain.DigitalHumanTask, actualDurationSeconds int) {
	updated, err := w.app.Store.SettleDigitalHumanTask(ctx, task.ID, actualDurationSeconds)
	if err == nil {
		if updated != nil {
			task = updated
		}
		return
	}

	finalCredits := w.finalCreditsForTask(task, actualDurationSeconds)
	message := fmt.Sprintf("成品已生成，但积分结算失败：%s，系统稍后会自动重试结算", buildFailureMessage(err))
	if _, markErr := w.app.Store.MarkDigitalHumanTaskSettlementPending(ctx, task.ID, &actualDurationSeconds, finalCredits, message); markErr != nil {
		w.app.Logger.Error("digital human worker failed to keep task in settlement pending state", "task_id", task.ID, "error", markErr)
		return
	}
	w.app.Logger.Warn("digital human worker deferred task settlement", "task_id", task.ID, "error", err)
}

func (w *Worker) submitTask(ctx context.Context, task *domain.DigitalHumanTask, leaseToken string) (*domain.DigitalHumanTask, error) {
	tempDir, request, err := w.materializeTaskAssets(ctx, task)
	if err != nil {
		if tempDir != "" {
			cleanupWorkingDir(w.app.Logger, tempDir)
		}
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), nil, nil)
		return nil, err
	}

	response, rawResponse, err := w.client.GenerateVideoAsync(ctx, request)
	if err != nil {
		cleanupWorkingDir(w.app.Logger, tempDir)
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), nil, rawResponse)
		return nil, err
	}

	progress := mustJSON(domain.DigitalHumanProgress{
		Current:    0,
		Total:      0,
		Percentage: 0,
		Message:    firstNonEmptyString(response.Message, "数字人任务已提交，等待生成"),
	})
	updated, syncErr := w.app.Store.SyncDigitalHumanTaskExecution(ctx, task.ID, leaseToken, store.UpdateDigitalHumanTaskExecutionInput{
		Status:                stringPtr("running"),
		RemoteTaskID:          stringPtr(response.TaskID),
		RemoteTaskTouched:     true,
		Progress:              progress,
		ProgressTouched:       true,
		RemoteResponsePayload: rawResponse,
		RemotePayloadTouched:  true,
		StartedAt:             timePtr(time.Now().UTC()),
		StartedTouched:        true,
		WorkingDir:            stringPtr(tempDir),
		WorkingDirTouched:     true,
	})
	if syncErr != nil {
		return nil, syncErr
	}
	return updated, nil
}

func (w *Worker) materializeTaskAssets(ctx context.Context, task *domain.DigitalHumanTask) (string, GenerateRequest, error) {
	tempDir, err := os.MkdirTemp("", "omnidrive-digital-human-*")
	if err != nil {
		return "", GenerateRequest{}, err
	}

	characterPath, err := w.writeTempAsset(ctx, tempDir, "character", task.CharacterAsset)
	if err != nil {
		return tempDir, GenerateRequest{}, err
	}
	audioPath, err := w.writeTempAsset(ctx, tempDir, "audio", task.RefAudioAsset)
	if err != nil {
		return tempDir, GenerateRequest{}, err
	}

	mode := strings.ToLower(strings.TrimSpace(task.Mode))
	switch mode {
	case "digital", "customize":
	default:
		return tempDir, GenerateRequest{}, fmt.Errorf("unsupported digital human mode: %s", task.Mode)
	}
	goodsText := strings.TrimSpace(task.GoodsText)
	if goodsText == "" {
		return tempDir, GenerateRequest{}, fmt.Errorf("digital human goods text is required")
	}

	request := GenerateRequest{
		CharacterAssetPath: characterPath,
		Mode:               mode,
		Source:             "runninghub",
		GoodsText:          goodsText,
		RefAudio:           stringPtr(audioPath),
	}
	if strings.TrimSpace(task.ModelName) != "" {
		request.LLMModel = stringPtr(strings.TrimSpace(task.ModelName))
	}
	if mode == "digital" {
		if task.GoodsTitle == nil || strings.TrimSpace(*task.GoodsTitle) == "" {
			return tempDir, GenerateRequest{}, fmt.Errorf("digital human digital mode requires goods title")
		}
		if task.GoodsAsset == nil {
			return tempDir, GenerateRequest{}, fmt.Errorf("digital human digital mode requires goods asset")
		}
		request.GoodsTitle = stringPtr(strings.TrimSpace(*task.GoodsTitle))

		goodsPath, goodsErr := w.writeTempAsset(ctx, tempDir, "goods", *task.GoodsAsset)
		if goodsErr != nil {
			return tempDir, GenerateRequest{}, goodsErr
		}
		request.GoodsAssetPath = stringPtr(goodsPath)
	}
	return tempDir, request, nil
}

func (w *Worker) writeTempAsset(ctx context.Context, tempDir string, prefix string, asset domain.DigitalHumanAsset) (string, error) {
	if strings.TrimSpace(asset.StorageKey) == "" {
		return "", fmt.Errorf("%s asset missing storage key", prefix)
	}
	data, _, err := w.app.Storage.ReadBytes(ctx, asset.StorageKey)
	if err != nil {
		return "", fmt.Errorf("read %s asset: %w", prefix, err)
	}
	fileName := sanitizeFileName(asset.FileName, prefix)
	targetPath := filepath.Join(tempDir, prefix+"-"+fileName)
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s temp asset: %w", prefix, err)
	}
	return targetPath, nil
}

func (w *Worker) saveResultAsset(ctx context.Context, task *domain.DigitalHumanTask, remoteTask *RemoteTask) (*resultSaveOutcome, error) {
	videoURL := strings.TrimSpace(extractResultVideoURL(remoteTask.Result))
	if videoURL == "" {
		return nil, fmt.Errorf("digital human result missing video_url")
	}
	resolvedVideoURL, err := w.resolveResultVideoURL(videoURL)
	if err != nil {
		return nil, err
	}

	workingDir := valueOrEmptyString(task.WorkingDir)
	cleanupTempDir := false
	if workingDir == "" {
		tempDir, err := os.MkdirTemp("", "omnidrive-digital-human-result-*")
		if err != nil {
			return nil, err
		}
		workingDir = tempDir
		cleanupTempDir = true
	}
	if cleanupTempDir {
		defer cleanupWorkingDir(w.app.Logger, workingDir)
	}

	targetPath := filepath.Join(workingDir, fmt.Sprintf("result-%s.mp4", uuid.NewString()))
	if err := w.downloadResultVideo(ctx, resolvedVideoURL, targetPath); err != nil {
		return nil, err
	}

	durationSeconds, probeErr := w.probeVideoDurationSeconds(ctx, targetPath)
	actualDurationSeconds := normalizeDurationSeconds(durationSeconds)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}
	contentType := strings.TrimSpace(http.DetectContentType(data))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "video/mp4"
	}
	fileName := resultFileName(resolvedVideoURL)
	object, err := w.app.Storage.SaveBytes(
		ctx,
		fmt.Sprintf("digital-human/%s/%s/result/%s-%s", task.OwnerUserID, task.ID, uuid.NewString(), fileName),
		contentType,
		data,
	)
	if err != nil {
		return nil, err
	}
	return &resultSaveOutcome{
		asset: &domain.DigitalHumanAsset{
			StorageKey: object.StorageKey,
			PublicURL:  object.PublicURL,
			FileName:   fileName,
			MimeType:   object.ContentType,
			SizeBytes:  int64Ptr(object.SizeBytes),
		},
		actualDurationSeconds: actualDurationSeconds,
		probeErr:              probeErr,
	}, nil
}

func (w *Worker) resolveResultVideoURL(rawURL string) (string, error) {
	if w.client != nil {
		resolved, err := w.client.ResolveURL(rawURL)
		if err == nil {
			return resolved, nil
		}
		return "", fmt.Errorf("resolve digital human result url %q: %w", strings.TrimSpace(rawURL), err)
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("resolve digital human result url %q: %w", strings.TrimSpace(rawURL), err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("resolve digital human result url %q: missing scheme or host", strings.TrimSpace(rawURL))
	}
	return parsed.String(), nil
}

func (w *Worker) failTask(ctx context.Context, task *domain.DigitalHumanTask, leaseToken string, message string, workingDir *string, rawResponse []byte) {
	if workingDir != nil {
		cleanupWorkingDir(w.app.Logger, *workingDir)
	}
	updated, err := w.app.Store.SyncDigitalHumanTaskExecution(ctx, task.ID, leaseToken, store.UpdateDigitalHumanTaskExecutionInput{
		Status:                stringPtr("failed"),
		RemoteResponsePayload: rawResponse,
		RemotePayloadTouched:  rawResponse != nil,
		ErrorMessage:          stringPtr(strings.TrimSpace(message)),
		CompletedAt:           timePtr(time.Now().UTC()),
		CompletedTouched:      true,
		WorkingDir:            nil,
		WorkingDirTouched:     true,
	})
	if err != nil {
		w.app.Logger.Error("digital human worker failed to mark task failed", "task_id", task.ID, "error", err)
		return
	}
	if updated != nil {
		task = updated
	}
	if _, refundErr := w.app.Store.RefundDigitalHumanTaskOnFailure(ctx, task.ID, strings.TrimSpace(message)); refundErr != nil {
		w.app.Logger.Error("digital human worker failed to refund failed task", "task_id", task.ID, "error", refundErr)
	}
}

func (w *Worker) probeStoredResultDuration(ctx context.Context, task *domain.DigitalHumanTask) (*int, error) {
	if task == nil || task.ResultAsset == nil {
		return nil, fmt.Errorf("result asset is missing")
	}

	tempDir, err := os.MkdirTemp("", "omnidrive-digital-human-settlement-*")
	if err != nil {
		return nil, err
	}
	defer cleanupWorkingDir(w.app.Logger, tempDir)

	resultPath, err := w.writeTempAsset(ctx, tempDir, "result", *task.ResultAsset)
	if err != nil {
		return nil, err
	}
	durationSeconds, err := w.probeVideoDurationSeconds(ctx, resultPath)
	if err != nil {
		return nil, err
	}
	return normalizeDurationSeconds(durationSeconds), nil
}

func (w *Worker) downloadResultVideo(ctx context.Context, rawURL string, targetPath string) error {
	if w.downloadVideo != nil {
		return w.downloadVideo(ctx, rawURL, targetPath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return err
	}

	httpClient := http.DefaultClient
	if w.client != nil && w.client.httpClient != nil {
		httpClient = w.client.httpClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download result video returned %d", resp.StatusCode)
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	return nil
}

func (w *Worker) probeVideoDurationSeconds(ctx context.Context, path string) (float64, error) {
	if w.probeDuration != nil {
		return w.probeDuration(ctx, path)
	}
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse ffprobe duration: %w", err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("ffprobe returned non-positive duration")
	}
	return parsed, nil
}

func (w *Worker) finalCreditsForTask(task *domain.DigitalHumanTask, actualDurationSeconds int) *int64 {
	creditsPerSecondMillis := int64(0)
	if task != nil && len(task.BillingPayload) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(task.BillingPayload, &payload); err == nil {
			if rawMillis, ok := payload["creditsPerSecondMillis"]; ok {
				switch value := rawMillis.(type) {
				case float64:
					creditsPerSecondMillis = int64(value)
				case int64:
					creditsPerSecondMillis = value
				case int:
					creditsPerSecondMillis = int64(value)
				case string:
					if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64); parseErr == nil {
						creditsPerSecondMillis = parsed
					}
				}
			}
			if creditsPerSecondMillis <= 0 {
				switch value := payload["creditsPerSecond"].(type) {
				case float64:
					parsed, parseErr := store.DigitalHumanCreditsToMillis(value)
					if parseErr == nil {
						creditsPerSecondMillis = parsed
					}
				case int64:
					parsed, parseErr := store.DigitalHumanCreditsToMillis(float64(value))
					if parseErr == nil {
						creditsPerSecondMillis = parsed
					}
				case int:
					parsed, parseErr := store.DigitalHumanCreditsToMillis(float64(value))
					if parseErr == nil {
						creditsPerSecondMillis = parsed
					}
				case string:
					if parsedFloat, parseErr := strconv.ParseFloat(strings.TrimSpace(value), 64); parseErr == nil {
						if parsed, convertErr := store.DigitalHumanCreditsToMillis(parsedFloat); convertErr == nil {
							creditsPerSecondMillis = parsed
						}
					}
				}
			}
		}
	}
	if creditsPerSecondMillis <= 0 && task != nil && task.EstimatedDurationSeconds > 0 && task.EstimatedCreditsMillis > 0 {
		creditsPerSecondMillis = task.EstimatedCreditsMillis / int64(task.EstimatedDurationSeconds)
	}
	if creditsPerSecondMillis <= 0 || actualDurationSeconds <= 0 {
		return nil
	}
	finalCredits := int64(actualDurationSeconds) * creditsPerSecondMillis
	return &finalCredits
}

func normalizeDurationSeconds(durationSeconds float64) *int {
	if durationSeconds <= 0 || math.IsNaN(durationSeconds) || math.IsInf(durationSeconds, 0) {
		return nil
	}
	seconds := int(math.Ceil(durationSeconds))
	if seconds <= 0 {
		seconds = 1
	}
	return &seconds
}

func resultFileName(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err == nil {
		base := sanitizeFileName(filepath.Base(parsed.Path), "result")
		if strings.Contains(base, ".") {
			return base
		}
	}
	return "result.mp4"
}

func (w *Worker) startLeaseHeartbeat(parent context.Context, taskID string, leaseToken string) func() {
	ctx, cancel := context.WithCancel(parent)
	ticker := time.NewTicker(digitalHumanLeaseTTL / 2)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.app.Store.RenewDigitalHumanTaskLease(ctx, taskID, leaseToken, time.Now().UTC().Add(digitalHumanLeaseTTL)); err != nil {
					w.app.Logger.Warn("digital human worker failed to renew task lease heartbeat", "task_id", taskID, "error", err)
				}
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

func buildProgressPayload(task *RemoteTask) domain.DigitalHumanProgress {
	if task != nil && task.Progress != nil {
		return domain.DigitalHumanProgress{
			Current:    task.Progress.Current,
			Total:      task.Progress.Total,
			Percentage: task.Progress.Percentage,
			Message:    strings.TrimSpace(task.Progress.Message),
		}
	}
	return domain.DigitalHumanProgress{
		Message: firstNonEmptyString(
			valueOrEmptyString(taskError(task)),
			fmt.Sprintf("数字人任务状态：%s", normalizeRemoteStatus(task.Status)),
		),
	}
}

func normalizeRemoteStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	case "pending", "running":
		return "running"
	default:
		return "running"
	}
}

func extractResultVideoURL(result any) string {
	typed, ok := result.(map[string]any)
	if !ok || len(typed) == 0 {
		return ""
	}
	if direct, ok := typed["video_url"].(string); ok {
		return strings.TrimSpace(direct)
	}
	if direct, ok := typed["videoUrl"].(string); ok {
		return strings.TrimSpace(direct)
	}
	if output, ok := typed["output"].(map[string]any); ok {
		if direct, ok := output["video_url"].(string); ok {
			return strings.TrimSpace(direct)
		}
		if direct, ok := output["videoUrl"].(string); ok {
			return strings.TrimSpace(direct)
		}
	}
	if video, ok := typed["video"].(map[string]any); ok {
		if direct, ok := video["url"].(string); ok {
			return strings.TrimSpace(direct)
		}
		if direct, ok := video["video_url"].(string); ok {
			return strings.TrimSpace(direct)
		}
		if direct, ok := video["videoUrl"].(string); ok {
			return strings.TrimSpace(direct)
		}
	}
	return ""
}

func sanitizeFileName(fileName string, fallback string) string {
	name := strings.TrimSpace(filepath.Base(fileName))
	if name == "" || name == "." || name == "/" {
		return fallback + defaultExtensionForMime("")
	}
	return strings.ReplaceAll(name, " ", "_")
}

func defaultExtensionForMime(mimeType string) string {
	if extensions, err := mime.ExtensionsByType(strings.TrimSpace(mimeType)); err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	return ".bin"
}

func buildFailureMessage(err error) string {
	if err == nil {
		return "数字人任务执行失败"
	}
	return strings.TrimSpace(err.Error())
}

func taskError(task *RemoteTask) *string {
	if task == nil || task.Error == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*task.Error)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mustJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func int64Ptr(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func valueOrEmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cleanupWorkingDir(logger *slog.Logger, workingDir string) {
	trimmed := strings.TrimSpace(workingDir)
	if trimmed == "" {
		return
	}
	if err := os.RemoveAll(trimmed); err != nil && logger != nil {
		logger.Warn("digital human worker failed to cleanup temp dir", "working_dir", trimmed, "error", err)
	}
}
