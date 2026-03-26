package ai

import "testing"

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
