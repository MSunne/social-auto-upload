package digitalhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
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
	app          *appstate.App
	client       *Client
	pollInterval time.Duration
	concurrency  int
	sem          chan struct{}
	activeTasks  sync.Map
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

	if claimed == nil || !strings.EqualFold(strings.TrimSpace(claimed.Status), "running") {
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
			resultAsset, resultErr := w.saveResultAsset(ctx, claimed, remoteTask)
			if resultErr != nil {
				w.failTask(ctx, claimed, leaseToken, buildFailureMessage(resultErr), claimed.WorkingDir, rawResponse)
				w.app.Logger.Error("digital human worker failed to mirror result", "task_id", claimed.ID, "error", resultErr)
				return
			}
			if claimed.WorkingDir != nil {
				cleanupWorkingDir(w.app.Logger, *claimed.WorkingDir)
			}
			updated, syncErr := w.app.Store.SyncDigitalHumanTaskExecution(ctx, claimed.ID, leaseToken, store.UpdateDigitalHumanTaskExecutionInput{
				Status:                stringPtr("completed"),
				ResultAsset:           mustJSON(resultAsset),
				ResultAssetTouched:    true,
				Progress:              progressJSON,
				ProgressTouched:       true,
				RemoteResponsePayload: rawResponse,
				RemotePayloadTouched:  true,
				CompletedAt:           timePtr(time.Now().UTC()),
				CompletedTouched:      true,
				WorkingDir:            nil,
				WorkingDirTouched:     true,
			})
			if syncErr != nil {
				w.app.Logger.Error("digital human worker failed to mark task complete", "task_id", claimed.ID, "error", syncErr)
			}
			if updated != nil {
				claimed = updated
			}
			return
		case "failed":
			w.failTask(ctx, claimed, leaseToken, firstNonEmptyString(valueOrEmptyString(remoteTask.Error), "数字人视频生成失败"), claimed.WorkingDir, rawResponse)
			return
		case "cancelled":
			if claimed.WorkingDir != nil {
				cleanupWorkingDir(w.app.Logger, *claimed.WorkingDir)
			}
			if _, err := w.app.Store.SyncDigitalHumanTaskExecution(ctx, claimed.ID, leaseToken, store.UpdateDigitalHumanTaskExecutionInput{
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
			}); err != nil {
				w.app.Logger.Error("digital human worker failed to mark task cancelled", "task_id", claimed.ID, "error", err)
			}
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

func (w *Worker) submitTask(ctx context.Context, task *domain.DigitalHumanTask, leaseToken string) (*domain.DigitalHumanTask, error) {
	tempDir, request, err := w.materializeTaskAssets(ctx, task)
	if err != nil {
		if tempDir != "" {
			cleanupWorkingDir(w.app.Logger, tempDir)
		}
		w.failTask(ctx, task, leaseToken, buildFailureMessage(err), nil, nil)
		return nil, err
	}

	response, rawResponse, err := w.client.GenerateVideo(ctx, request)
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

	request := GenerateRequest{
		CharacterAssetPath: characterPath,
		Mode:               task.Mode,
		GoodsText:          task.GoodsText,
		Source:             task.Source,
		RefAudio:           audioPath,
	}
	if task.GoodsTitle != nil {
		request.GoodsTitle = stringPtr(strings.TrimSpace(*task.GoodsTitle))
	}
	if task.GoodsAsset != nil {
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

func (w *Worker) saveResultAsset(ctx context.Context, task *domain.DigitalHumanTask, remoteTask *RemoteTask) (*domain.DigitalHumanAsset, error) {
	videoURL := strings.TrimSpace(extractResultVideoURL(remoteTask.Result))
	if videoURL == "" {
		return nil, fmt.Errorf("digital human result missing video_url")
	}
	object, err := w.app.Storage.SaveRemoteURL(
		ctx,
		fmt.Sprintf("digital-human/%s/%s/result/%s-result.mp4", task.OwnerUserID, task.ID, uuid.NewString()),
		"video/mp4",
		videoURL,
	)
	if err != nil {
		return nil, err
	}
	fileName := filepath.Base(strings.TrimSpace(object.StorageKey))
	return &domain.DigitalHumanAsset{
		StorageKey: object.StorageKey,
		PublicURL:  object.PublicURL,
		FileName:   fileName,
		MimeType:   object.ContentType,
		SizeBytes:  int64Ptr(object.SizeBytes),
	}, nil
}

func (w *Worker) failTask(ctx context.Context, task *domain.DigitalHumanTask, leaseToken string, message string, workingDir *string, rawResponse []byte) {
	if workingDir != nil {
		cleanupWorkingDir(w.app.Logger, *workingDir)
	}
	_, err := w.app.Store.SyncDigitalHumanTaskExecution(ctx, task.ID, leaseToken, store.UpdateDigitalHumanTaskExecutionInput{
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
	}
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

func extractResultVideoURL(result map[string]any) string {
	if len(result) == 0 {
		return ""
	}
	if direct, ok := result["video_url"].(string); ok {
		return strings.TrimSpace(direct)
	}
	if video, ok := result["video"].(map[string]any); ok {
		if direct, ok := video["url"].(string); ok {
			return strings.TrimSpace(direct)
		}
		if direct, ok := video["video_url"].(string); ok {
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
