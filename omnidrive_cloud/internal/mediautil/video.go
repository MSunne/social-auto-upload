package mediautil

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func CleanupWorkingDir(logger *slog.Logger, workingDir string, warningMessage string) {
	trimmed := strings.TrimSpace(workingDir)
	if trimmed == "" {
		return
	}
	if err := os.RemoveAll(trimmed); err != nil && logger != nil {
		logger.Warn(strings.TrimSpace(warningMessage), "working_dir", trimmed, "error", err)
	}
}

func ProbeVideoDurationSeconds(ctx context.Context, path string) (float64, error) {
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

func NormalizeDurationSeconds(durationSeconds float64) *int {
	if durationSeconds <= 0 || math.IsNaN(durationSeconds) || math.IsInf(durationSeconds, 0) {
		return nil
	}
	seconds := int(math.Ceil(durationSeconds))
	if seconds <= 0 {
		seconds = 1
	}
	return &seconds
}
