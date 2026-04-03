package ai

import (
	"errors"
	"os/exec"
	"testing"
)

func TestStandardizedVideoFileName(t *testing.T) {
	if got := standardizedVideoFileName("clip.webm"); got != "clip.mp4" {
		t.Fatalf("expected clip.mp4, got %s", got)
	}
	if got := standardizedVideoFileName(""); got != "video.mp4" {
		t.Fatalf("expected video.mp4 for empty input, got %s", got)
	}
}

func TestStandardizedVideoFilter(t *testing.T) {
	got := standardizedVideoFilter()
	want := "scale='max(2,trunc(iw*0.98/2)*2)':'max(2,trunc(ih*0.98/2)*2)',fps=30"
	if got != want {
		t.Fatalf("unexpected filter: %s", got)
	}
}

func TestStandardizedVideoOutputPathAvoidsInPlaceOverwrite(t *testing.T) {
	got := standardizedVideoOutputPath("/tmp/work", "video.mp4", "video.mp4")
	want := "/tmp/work/video.standardized.mp4"
	if got != want {
		t.Fatalf("unexpected output path: %s", got)
	}
}

func TestStandardizedVideoOutputPathKeepsDistinctOutputName(t *testing.T) {
	got := standardizedVideoOutputPath("/tmp/work", "video.webm", "video.mp4")
	want := "/tmp/work/video.mp4"
	if got != want {
		t.Fatalf("unexpected output path: %s", got)
	}
}

func TestFormatFFmpegExecutionErrorNotFound(t *testing.T) {
	got := formatFFmpegExecutionError(exec.ErrNotFound, nil, "/usr/local/bin/ffmpeg")
	want := "ffmpeg executable not found: /usr/local/bin/ffmpeg; install ffmpeg or set OMNIDRIVE_AI_VIDEO_STANDARDIZE_ENABLED=false"
	if got != want {
		t.Fatalf("unexpected not-found message: %s", got)
	}
}

func TestFormatFFmpegExecutionErrorWithOutput(t *testing.T) {
	err := errors.New("exit status 1")
	got := formatFFmpegExecutionError(err, []byte("codec failure"), "")
	want := "codec failure (binary=ffmpeg)"
	if got != want {
		t.Fatalf("unexpected formatted output message: %s", got)
	}
}
