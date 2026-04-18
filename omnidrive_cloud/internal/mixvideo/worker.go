package mixvideo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/mediautil"
	"omnidrive_cloud/internal/store"
)

const mixVideoLeaseTTL = 90 * time.Second

type Worker struct {
	app          *appstate.App
	client       *Client
	pollInterval time.Duration
	concurrency  int
	sem          chan struct{}
	activeTasks  sync.Map
}

type resultSaveOutcome struct {
	asset                 *domain.MixVideoAsset
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
	pollSeconds := app.Config.MixVideoPollSeconds
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

	w.app.Logger.Info("mix video worker started",
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
		w.app.Logger.Info("mix video worker stopped")
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
	tasks, err := w.app.Store.ListExecutableMixVideoTasks(ctx, limit)
	if err != nil {
		w.app.Logger.Error("mix video worker failed to list executable tasks", "error", err)
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

		go func(task domain.MixVideoTask) {
			defer func() {
				<-w.sem
				w.activeTasks.Delete(task.ID)
			}()
			w.processTask(ctx, task)
		}(task)
	}
}

func (w *Worker) processTask(ctx context.Context, task domain.MixVideoTask) {
	leaseToken := uuid.NewString()
	leaseExpiresAt := time.Now().UTC().Add(mixVideoLeaseTTL)

	claimed, err := w.app.Store.ClaimMixVideoTaskLease(ctx, task.ID, leaseToken, leaseExpiresAt)
	if err != nil {
		w.app.Logger.Error("mix video worker failed to claim task lease", "task_id", task.ID, "error", err)
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
			w.app.Logger.Error("mix video worker failed to submit task", "task_id", task.ID, "error", err)
			return
		}
	}
	if claimed == nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(claimed.Status), "completed") && strings.EqualFold(strings.TrimSpace(claimed.BillingStatus), "settlement_pending") {
		w.resumeSettlement(ctx, claimed)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(claimed.Status), "running") {
		return
	}
	if strings.TrimSpace(valueOrEmptyString(claimed.RemoteTaskID)) == "" {
		w.failTask(ctx, claimed, leaseToken, "混剪任务缺少远端 task_id", claimed.WorkingDir, nil)
		return
	}

	for {
		if ctx.Err() != nil {
			return
		}
		if _, err := w.app.Store.RenewMixVideoTaskLease(ctx, claimed.ID, leaseToken, time.Now().UTC().Add(mixVideoLeaseTTL)); err != nil {
			w.app.Logger.Error("mix video worker failed to renew task lease", "task_id", claimed.ID, "error", err)
			return
		}

		remoteTask, rawResponse, err := w.client.GetTask(ctx, valueOrEmptyString(claimed.RemoteTaskID))
		if err != nil {
			w.failTask(ctx, claimed, leaseToken, buildFailureMessage(err), claimed.WorkingDir, nil)
			w.app.Logger.Error("mix video worker failed to poll remote task", "task_id", claimed.ID, "error", err)
			return
		}

		progressPayload := buildProgressPayload(remoteTask)
		progressJSON := mustJSON(progressPayload)

		switch normalizeRemoteStatus(remoteTask.Status) {
		case "completed":
			w.completeTask(ctx, claimed, leaseToken, remoteTask, rawResponse, progressJSON)
			return
		case "failed":
			w.failTask(ctx, claimed, leaseToken, firstNonEmptyString(valueOrEmptyString(remoteTask.Error), "混剪视频生成失败"), claimed.WorkingDir, rawResponse)
			return
		case "cancelled":
			w.cancelTask(ctx, claimed, leaseToken, progressJSON, rawResponse)
			return
		default:
			updated, syncErr := w.app.Store.SyncMixVideoTaskExecution(ctx, claimed.ID, leaseToken, store.UpdateMixVideoTaskExecutionInput{
				Status:                stringPtr("running"),
				Progress:              progressJSON,
				ProgressTouched:       true,
				RemoteResponsePayload: rawResponse,
				RemotePayloadTouched:  true,
			})
			if syncErr != nil {
				w.app.Logger.Error("mix video worker failed to sync running task", "task_id", claimed.ID, "error", syncErr)
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

func (w *Worker) submitTask(ctx context.Context, task *domain.MixVideoTask, leaseToken string) (*domain.MixVideoTask, error) {
	tempDir, request, err := w.materializeTaskAssets(ctx, task)
	if err != nil {
		if tempDir != "" {
			mediautil.CleanupWorkingDir(w.app.Logger, tempDir, "mix video worker failed to cleanup temp dir")
		}
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), nil, nil)
		return nil, err
	}

	requestPayload, err := json.Marshal(map[string]any{
		"sourceRequest": compactJSONPayload(task.RequestPayload),
		"submissionRequest": map[string]any{
			"assetCount": len(request.AssetPaths),
			"refAudio":   filepath.Base(request.RefAudioPath),
			"scriptText": strings.TrimSpace(request.ScriptText),
		},
	})
	if err != nil {
		mediautil.CleanupWorkingDir(w.app.Logger, tempDir, "mix video worker failed to cleanup temp dir")
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), nil, nil)
		return nil, err
	}

	startedAt := time.Now().UTC()
	progress := mustJSON(domain.MixVideoProgress{
		Message: "正在提交到混剪生成服务",
	})
	updated, syncErr := w.app.Store.SyncMixVideoTaskExecution(ctx, task.ID, leaseToken, store.UpdateMixVideoTaskExecutionInput{
		Status:                stringPtr("running"),
		RequestPayload:        requestPayload,
		RequestPayloadTouched: true,
		Progress:              progress,
		ProgressTouched:       true,
		StartedAt:             timePtr(startedAt),
		StartedTouched:        true,
		WorkingDir:            stringPtr(tempDir),
		WorkingDirTouched:     true,
	})
	if syncErr != nil {
		mediautil.CleanupWorkingDir(w.app.Logger, tempDir, "mix video worker failed to cleanup temp dir")
		return nil, syncErr
	}
	if updated != nil {
		task = updated
	}

	response, rawResponse, err := w.client.GenerateAsync(ctx, request)
	if err != nil {
		mediautil.CleanupWorkingDir(w.app.Logger, tempDir, "mix video worker failed to cleanup temp dir")
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), nil, rawResponse)
		return nil, err
	}

	progress = mustJSON(domain.MixVideoProgress{
		Message: firstNonEmptyString(response.Message, "混剪任务已提交，等待生成"),
	})
	updated, syncErr = w.app.Store.SyncMixVideoTaskExecution(ctx, task.ID, leaseToken, store.UpdateMixVideoTaskExecutionInput{
		Status:                stringPtr("running"),
		RemoteTaskID:          stringPtr(response.TaskID),
		RemoteTaskTouched:     true,
		Progress:              progress,
		ProgressTouched:       true,
		RemoteResponsePayload: rawResponse,
		RemotePayloadTouched:  true,
	})
	if syncErr != nil {
		return nil, syncErr
	}
	return updated, nil
}

func (w *Worker) materializeTaskAssets(ctx context.Context, task *domain.MixVideoTask) (string, GenerateRequest, error) {
	tempDir, err := os.MkdirTemp("", "omnidrive-mix-video-*")
	if err != nil {
		return "", GenerateRequest{}, err
	}
	if len(task.SourceAssets) == 0 {
		return tempDir, GenerateRequest{}, fmt.Errorf("mix video assets are required")
	}

	assetPaths := make([]string, 0, len(task.SourceAssets))
	for idx, asset := range task.SourceAssets {
		path, writeErr := w.writeTempAsset(ctx, tempDir, fmt.Sprintf("asset-%02d", idx+1), asset)
		if writeErr != nil {
			return tempDir, GenerateRequest{}, writeErr
		}
		assetPaths = append(assetPaths, path)
	}
	audioPath, err := w.writeTempAsset(ctx, tempDir, "audio", task.RefAudioAsset)
	if err != nil {
		return tempDir, GenerateRequest{}, err
	}
	scriptText := strings.TrimSpace(task.ScriptText)
	if scriptText == "" {
		return tempDir, GenerateRequest{}, fmt.Errorf("mix video script text is required")
	}
	return tempDir, GenerateRequest{
		AssetPaths:   assetPaths,
		RefAudioPath: audioPath,
		ScriptText:   scriptText,
	}, nil
}

func (w *Worker) writeTempAsset(ctx context.Context, tempDir string, prefix string, asset domain.MixVideoAsset) (string, error) {
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

func (w *Worker) completeTask(ctx context.Context, task *domain.MixVideoTask, leaseToken string, remoteTask *RemoteTask, rawResponse []byte, progressJSON []byte) {
	outcome, err := w.saveResultAsset(ctx, task, remoteTask)
	if err != nil {
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), task.WorkingDir, rawResponse)
		w.app.Logger.Error("mix video worker failed to mirror result", "task_id", task.ID, "error", err)
		return
	}
	if task.WorkingDir != nil {
		mediautil.CleanupWorkingDir(w.app.Logger, *task.WorkingDir, "mix video worker failed to cleanup temp dir")
	}

	input := store.UpdateMixVideoTaskExecutionInput{
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

	updated, syncErr := w.app.Store.SyncMixVideoTaskExecution(ctx, task.ID, leaseToken, input)
	if syncErr != nil {
		w.app.Logger.Error("mix video worker failed to mark task complete", "task_id", task.ID, "error", syncErr)
		return
	}
	if updated != nil {
		task = updated
	}

	if outcome.probeErr != nil {
		message := fmt.Sprintf("成品已生成，但读取视频时长失败：%s，系统稍后会自动重试结算", buildFailureMessage(outcome.probeErr))
		if _, markErr := w.app.Store.MarkMixVideoTaskSettlementPending(ctx, task.ID, nil, nil, message); markErr != nil {
			w.app.Logger.Error("mix video worker failed to mark settlement pending after probe failure", "task_id", task.ID, "error", markErr)
		}
		return
	}
	if outcome.actualDurationSeconds == nil {
		if _, markErr := w.app.Store.MarkMixVideoTaskSettlementPending(ctx, task.ID, nil, nil, "成品已生成，但缺少实际时长，系统稍后会自动重试结算"); markErr != nil {
			w.app.Logger.Error("mix video worker failed to mark settlement pending without duration", "task_id", task.ID, "error", markErr)
		}
		return
	}

	w.settleCompletedTask(ctx, task, *outcome.actualDurationSeconds)
}

func (w *Worker) saveResultAsset(ctx context.Context, task *domain.MixVideoTask, remoteTask *RemoteTask) (*resultSaveOutcome, error) {
	videoURL := strings.TrimSpace(extractResultVideoURL(remoteTask.Result))
	if videoURL == "" {
		return nil, fmt.Errorf("mix video result missing video url")
	}
	resolvedVideoURL, err := w.resolveResultVideoURL(videoURL)
	if err != nil {
		return nil, err
	}

	workingDir := valueOrEmptyString(task.WorkingDir)
	cleanupTempDir := false
	if workingDir == "" {
		tempDir, err := os.MkdirTemp("", "omnidrive-mix-video-result-*")
		if err != nil {
			return nil, err
		}
		workingDir = tempDir
		cleanupTempDir = true
	}
	if cleanupTempDir {
		defer mediautil.CleanupWorkingDir(w.app.Logger, workingDir, "mix video worker failed to cleanup temp dir")
	}

	targetPath := filepath.Join(workingDir, fmt.Sprintf("result-%s.mp4", uuid.NewString()))
	if err := w.downloadResultVideo(ctx, resolvedVideoURL, targetPath); err != nil {
		return nil, err
	}

	durationSeconds, probeErr := mediautil.ProbeVideoDurationSeconds(ctx, targetPath)
	actualDurationSeconds := mediautil.NormalizeDurationSeconds(durationSeconds)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}
	contentType := strings.TrimSpace(http.DetectContentType(data))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "video/mp4"
	}
	fileName := resultFileName(resolvedVideoURL)
	object, err := w.app.Storage.SaveBytes(ctx, fmt.Sprintf("mix-video/%s/%s/result/%s-%s", task.OwnerUserID, task.ID, uuid.NewString(), fileName), contentType, data)
	if err != nil {
		return nil, err
	}
	return &resultSaveOutcome{
		asset: &domain.MixVideoAsset{
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
	if localPath, ok, err := resolveLocalResultVideoPath(rawURL); err != nil {
		return "", fmt.Errorf("resolve mix video result url %q: %w", strings.TrimSpace(rawURL), err)
	} else if ok {
		return localPath, nil
	}
	resolved, err := w.client.ResolveURL(rawURL)
	if err == nil {
		return resolved, nil
	}
	return "", fmt.Errorf("resolve mix video result url %q: %w", strings.TrimSpace(rawURL), err)
}

func (w *Worker) downloadResultVideo(ctx context.Context, rawURL string, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	if localPath, ok, err := resolveLocalResultVideoPath(rawURL); err != nil {
		return err
	} else if ok {
		return copyLocalResultVideo(localPath, targetPath)
	}
	if w.client == nil || w.client.httpClient == nil {
		return fmt.Errorf("mix video http client is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return err
	}
	resp, err := w.client.httpClient.Do(req)
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
	_, err = io.Copy(file, resp.Body)
	return err
}

func resolveLocalResultVideoPath(rawURL string) (string, bool, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false, nil
	}
	if strings.HasPrefix(trimmed, "/api/") {
		return "", false, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false, err
	}
	switch parsed.Scheme {
	case "":
		if filepath.IsAbs(trimmed) {
			return trimmed, true, nil
		}
		return "", false, nil
	case "file":
		if parsed.Path == "" {
			return "", false, fmt.Errorf("mix video local result path is empty")
		}
		return parsed.Path, true, nil
	default:
		return "", false, nil
	}
}

func copyLocalResultVideo(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(strings.TrimSpace(sourcePath))
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	return err
}

func (w *Worker) failTask(ctx context.Context, task *domain.MixVideoTask, leaseToken string, message string, workingDir *string, rawResponse []byte) {
	if workingDir != nil {
		mediautil.CleanupWorkingDir(w.app.Logger, *workingDir, "mix video worker failed to cleanup temp dir")
	}
	updated, err := w.app.Store.SyncMixVideoTaskExecution(ctx, task.ID, leaseToken, store.UpdateMixVideoTaskExecutionInput{
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
		w.app.Logger.Error("mix video worker failed to mark task failed", "task_id", task.ID, "error", err)
		return
	}
	if updated != nil {
		task = updated
	}
	if _, refundErr := w.app.Store.RefundMixVideoTaskOnFailure(ctx, task.ID, strings.TrimSpace(message)); refundErr != nil {
		w.app.Logger.Error("mix video worker failed to refund failed task", "task_id", task.ID, "error", refundErr)
	}
}

func (w *Worker) cancelTask(ctx context.Context, task *domain.MixVideoTask, leaseToken string, progressJSON []byte, rawResponse []byte) {
	if task.WorkingDir != nil {
		mediautil.CleanupWorkingDir(w.app.Logger, *task.WorkingDir, "mix video worker failed to cleanup temp dir")
	}
	updated, err := w.app.Store.SyncMixVideoTaskExecution(ctx, task.ID, leaseToken, store.UpdateMixVideoTaskExecutionInput{
		Status:                stringPtr("cancelled"),
		Progress:              progressJSON,
		ProgressTouched:       true,
		RemoteResponsePayload: rawResponse,
		RemotePayloadTouched:  true,
		ErrorMessage:          stringPtr("混剪任务已取消"),
		CompletedAt:           timePtr(time.Now().UTC()),
		CompletedTouched:      true,
		WorkingDir:            nil,
		WorkingDirTouched:     true,
	})
	if err != nil {
		w.app.Logger.Error("mix video worker failed to mark task cancelled", "task_id", task.ID, "error", err)
		return
	}
	if updated != nil {
		task = updated
	}
	if _, refundErr := w.app.Store.RefundMixVideoTaskOnFailure(ctx, task.ID, "混剪任务已取消，已退回预扣积分"); refundErr != nil {
		w.app.Logger.Error("mix video worker failed to refund cancelled task", "task_id", task.ID, "error", refundErr)
	}
}

func (w *Worker) resumeSettlement(ctx context.Context, task *domain.MixVideoTask) {
	actualDurationSeconds := task.ActualDurationSeconds
	if actualDurationSeconds == nil {
		probedDuration, err := w.probeStoredResultDuration(ctx, task)
		if err != nil {
			message := fmt.Sprintf("成品已生成，但读取视频时长失败：%s，系统稍后会自动重试结算", buildFailureMessage(err))
			if _, markErr := w.app.Store.MarkMixVideoTaskSettlementPending(ctx, task.ID, nil, nil, message); markErr != nil {
				w.app.Logger.Error("mix video worker failed to refresh settlement pending state", "task_id", task.ID, "error", markErr)
			}
			return
		}
		actualDurationSeconds = probedDuration
	}
	w.settleCompletedTask(ctx, task, *actualDurationSeconds)
}

func (w *Worker) probeStoredResultDuration(ctx context.Context, task *domain.MixVideoTask) (*int, error) {
	if task == nil || task.ResultAsset == nil {
		return nil, fmt.Errorf("result asset is missing")
	}
	tempDir, err := os.MkdirTemp("", "omnidrive-mix-video-settlement-*")
	if err != nil {
		return nil, err
	}
	defer mediautil.CleanupWorkingDir(w.app.Logger, tempDir, "mix video worker failed to cleanup temp dir")

	resultPath, err := w.writeTempAsset(ctx, tempDir, "result", *task.ResultAsset)
	if err != nil {
		return nil, err
	}
	durationSeconds, err := mediautil.ProbeVideoDurationSeconds(ctx, resultPath)
	if err != nil {
		return nil, err
	}
	return mediautil.NormalizeDurationSeconds(durationSeconds), nil
}

func (w *Worker) settleCompletedTask(ctx context.Context, task *domain.MixVideoTask, actualDurationSeconds int) {
	updated, err := w.app.Store.SettleMixVideoTask(ctx, task.ID, actualDurationSeconds)
	if err == nil {
		if updated != nil {
			task = updated
		}
		return
	}
	finalCredits := w.finalCreditsForTask(task, actualDurationSeconds)
	message := fmt.Sprintf("成品已生成，但积分结算失败：%s，系统稍后会自动重试结算", buildFailureMessage(err))
	if _, markErr := w.app.Store.MarkMixVideoTaskSettlementPending(ctx, task.ID, &actualDurationSeconds, finalCredits, message); markErr != nil {
		w.app.Logger.Error("mix video worker failed to keep task in settlement pending state", "task_id", task.ID, "error", markErr)
		return
	}
	w.app.Logger.Warn("mix video worker deferred task settlement", "task_id", task.ID, "error", err)
}

func (w *Worker) finalCreditsForTask(task *domain.MixVideoTask, actualDurationSeconds int) *int64 {
	var payload map[string]any
	if task != nil && len(task.BillingPayload) > 0 {
		_ = json.Unmarshal(task.BillingPayload, &payload)
	}
	creditsPerSecondMillis := store.CreditMillisScale
	if parsed := storeCreditsPerSecondMillis(payload, task); parsed > 0 {
		creditsPerSecondMillis = parsed
	}
	if actualDurationSeconds <= 0 || creditsPerSecondMillis <= 0 {
		return nil
	}
	finalCredits := int64(actualDurationSeconds) * creditsPerSecondMillis
	return &finalCredits
}

func storeCreditsPerSecondMillis(payload map[string]any, task *domain.MixVideoTask) int64 {
	if payload != nil {
		if parsed, ok := parsePositiveInt64(payload["creditsPerSecondMillis"]); ok {
			return parsed
		}
		if parsed, ok := parseCreditMillis(payload["creditsPerSecond"]); ok {
			return parsed
		}
	}
	if task != nil && task.EstimatedDurationSeconds > 0 && task.EstimatedCreditsMillis > 0 {
		return task.EstimatedCreditsMillis / int64(task.EstimatedDurationSeconds)
	}
	return 0
}

func (w *Worker) startLeaseHeartbeat(parent context.Context, taskID string, leaseToken string) func() {
	ctx, cancel := context.WithCancel(parent)
	ticker := time.NewTicker(mixVideoLeaseTTL / 2)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.app.Store.RenewMixVideoTaskLease(ctx, taskID, leaseToken, time.Now().UTC().Add(mixVideoLeaseTTL)); err != nil {
					w.app.Logger.Warn("mix video worker failed to renew task lease heartbeat", "task_id", taskID, "error", err)
				}
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

func buildProgressPayload(task *RemoteTask) domain.MixVideoProgress {
	if task != nil && task.Progress != nil {
		return domain.MixVideoProgress{
			Current:    task.Progress.Current,
			Total:      task.Progress.Total,
			Percentage: task.Progress.Percentage,
			Message:    strings.TrimSpace(task.Progress.Message),
		}
	}
	return domain.MixVideoProgress{
		Message: firstNonEmptyString(valueOrEmptyString(taskError(task)), fmt.Sprintf("混剪任务状态：%s", normalizeRemoteStatus(task.Status))),
	}
}

func normalizeRemoteStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "success", "succeeded":
		return "completed"
	case "failed", "error":
		return "failed"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return "running"
	}
}

func extractResultVideoURL(result any) string {
	switch typed := result.(type) {
	case map[string]any:
		for _, key := range []string{"video_url", "videoUrl", "output_url", "outputUrl", "url", "download_url"} {
			if direct, ok := typed[key].(string); ok && strings.TrimSpace(direct) != "" {
				return strings.TrimSpace(direct)
			}
		}
		for _, value := range typed {
			if nested := extractResultVideoURL(value); nested != "" {
				return nested
			}
		}
	case []any:
		for _, item := range typed {
			if nested := extractResultVideoURL(item); nested != "" {
				return nested
			}
		}
	}
	return ""
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

func sanitizeFileName(fileName string, fallback string) string {
	name := strings.TrimSpace(filepath.Base(fileName))
	if name == "" || name == "." || name == "/" {
		return fallback + ".bin"
	}
	return strings.ReplaceAll(name, " ", "_")
}

func buildFailureMessage(err error) string {
	if err == nil {
		return "混剪任务执行失败"
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

func compactJSONPayload(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	compact, err := json.Marshal(decoded)
	if err != nil {
		return nil
	}
	return compact
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parsePositiveInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, typed > 0
	case int:
		return int64(typed), typed > 0
	case float64:
		parsed := int64(typed)
		return parsed, parsed > 0
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil && parsed > 0
	}
	return 0, false
}

func parseCreditMillis(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		parsed, err := store.CreditsToMillis(typed)
		return parsed, err == nil && parsed > 0
	case int64:
		parsed, err := store.CreditsToMillis(float64(typed))
		return parsed, err == nil && parsed > 0
	case int:
		parsed, err := store.CreditsToMillis(float64(typed))
		return parsed, err == nil && parsed > 0
	}
	return 0, false
}
