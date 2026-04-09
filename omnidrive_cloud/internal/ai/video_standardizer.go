package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	standardizedVideoFPS        = 30
	standardizedVideoScaleRatio = 0.98
	standardizedVideoCRF        = 20
)

// 处理standardize视频产物相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func standardizeVideoArtifact(ctx context.Context, artifact BinaryArtifact, ffmpegPath string) (BinaryArtifact, error) {
	inputBytes := artifact.Data
	if len(inputBytes) == 0 {
		return BinaryArtifact{}, fmt.Errorf("video artifact data is empty")
	}

	tempDir, err := os.MkdirTemp("", "omnidrive-video-standardize-*")
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputName := safeFileName(artifact.FileName)
	if inputName == "" {
		inputName = "input.mp4"
	}
	inputPath := filepath.Join(tempDir, inputName)
	outputFileName := standardizedVideoFileName(artifact.FileName)
	outputPath := standardizedVideoOutputPath(tempDir, inputName, outputFileName)

	if err := os.WriteFile(inputPath, inputBytes, 0o600); err != nil {
		return BinaryArtifact{}, fmt.Errorf("write temp input video: %w", err)
	}

	command := exec.CommandContext(
		ctx,
		ffmpegBinary(ffmpegPath),
		"-y",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-vf", standardizedVideoFilter(),
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", fmt.Sprintf("%d", standardizedVideoCRF),
		"-pix_fmt", "yuv420p",
		"-r", fmt.Sprintf("%d", standardizedVideoFPS),
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		outputPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("ffmpeg standardize video failed: %s", formatFFmpegExecutionError(err, output, ffmpegPath))
	}

	normalizedBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("read temp output video: %w", err)
	}

	metadata := cloneArtifactMetadata(artifact.Metadata)
	metadata["postprocess"] = map[string]any{
		"videoCodec":  "h264",
		"fps":         standardizedVideoFPS,
		"scaleRatio":  standardizedVideoScaleRatio,
		"pixelFormat": "yuv420p",
	}

	return BinaryArtifact{
		FileName:    outputFileName,
		MIMEType:    "video/mp4",
		Data:        normalizedBytes,
		SizeBytes:   int64(len(normalizedBytes)),
		Metadata:    metadata,
		ArtifactKey: artifact.ArtifactKey,
	}, nil
}

// 处理ffmpegBinary相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func ffmpegBinary(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "ffmpeg"
	}
	return trimmed
}

// 处理formatFFmpegExecution错误相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func formatFFmpegExecutionError(err error, output []byte, ffmpegPath string) string {
	binary := ffmpegBinary(ffmpegPath)
	message := strings.TrimSpace(string(output))

	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Sprintf(
			"ffmpeg executable not found: %s; install ffmpeg or set OMNIDRIVE_AI_VIDEO_STANDARDIZE_ENABLED=false",
			binary,
		)
	}

	if message == "" {
		message = err.Error()
	}
	return fmt.Sprintf("%s (binary=%s)", message, binary)
}

// 处理standardized视频文件名称相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func standardizedVideoFileName(fileName string) string {
	cleaned := safeFileName(fileName)
	if cleaned == "" {
		return "video.mp4"
	}
	ext := filepath.Ext(cleaned)
	base := strings.TrimSuffix(cleaned, ext)
	if base == "" {
		base = "video"
	}
	return base + ".mp4"
}

// 处理standardized视频输出路径相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func standardizedVideoOutputPath(tempDir string, inputName string, outputFileName string) string {
	outputPath := filepath.Join(tempDir, outputFileName)
	if filepath.Clean(outputPath) != filepath.Clean(filepath.Join(tempDir, inputName)) {
		return outputPath
	}

	ext := filepath.Ext(outputFileName)
	base := strings.TrimSuffix(outputFileName, ext)
	if base == "" {
		base = "video"
	}
	return filepath.Join(tempDir, base+".standardized"+ext)
}

// 处理standardized视频Filter相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func standardizedVideoFilter() string {
	return fmt.Sprintf(
		"scale='max(2,trunc(iw*%.2f/2)*2)':'max(2,trunc(ih*%.2f/2)*2)',fps=%d",
		standardizedVideoScaleRatio,
		standardizedVideoScaleRatio,
		standardizedVideoFPS,
	)
}

// 处理clone产物Metadata相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func cloneArtifactMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
