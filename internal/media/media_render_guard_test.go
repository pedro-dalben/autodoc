package media_test

import (
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/timeline"
)

func timelineWithScenes(t *testing.T, dir string) *timeline.Timeline {
	t.Helper()
	tl := testTimeline(t, dir)
	tl.SceneClips = []timeline.SceneClip{
		{SceneID: "scene-001", StartS: 0, EndS: tl.TotalS / 2, Duration: tl.TotalS / 2},
		{SceneID: "scene-002", StartS: tl.TotalS / 2, EndS: tl.TotalS, Duration: tl.TotalS - tl.TotalS/2},
	}
	return tl
}

// Silent solid-color output is worse than an error: render must fail hard
// when a timeline with scene clips has no raw video at all.
func TestBuildFinalMP4FailsOnEmptySceneVideo(t *testing.T) {
	dir := t.TempDir()
	tl := timelineWithScenes(t, dir)
	err := media.BuildFinalMP4(tl, media.RenderOptions{
		Width: 640, Height: 360, FPS: 15,
		OutputMP4: dir + "/tutorial.mp4",
	})
	if err == nil {
		t.Fatal("expected error for empty SceneVideo, got nil (would render black MP4)")
	}
	if !strings.Contains(err.Error(), "scene-001") && !strings.Contains(strings.ToLower(err.Error()), "no scene video") {
		t.Fatalf("error should name the missing scene(s): %v", err)
	}
}

// A partial map (one scene missing its raw) must also fail, not render the
// missing scene as a flat 0x1a1d29 stretch.
func TestBuildFinalMP4FailsOnPartialSceneVideo(t *testing.T) {
	dir := t.TempDir()
	tl := timelineWithScenes(t, dir)
	err := media.BuildFinalMP4(tl, media.RenderOptions{
		Width: 640, Height: 360, FPS: 15,
		SceneVideo: map[string]string{"scene-001": dir + "/raw.webm"},
		OutputMP4:  dir + "/tutorial.mp4",
	})
	if err == nil {
		t.Fatal("expected error for missing scene-002, got nil")
	}
	if !strings.Contains(err.Error(), "scene-002") {
		t.Fatalf("error should name scene-002: %v", err)
	}
}

// Audio-only timelines (no scene clips, e.g. unit fixtures) keep the legacy
// color fallback so existing callers/tests are unaffected.
func TestBuildFinalMP4AllowsAudioOnlyTimeline(t *testing.T) {
	if _, err := media.CheckFFmpeg(); err != nil {
		t.Skip("ffmpeg missing")
	}
	dir := t.TempDir()
	tl := testTimeline(t, dir)
	if len(tl.SceneClips) != 0 {
		t.Fatal("fixture timeline should have no scene clips")
	}
	if err := media.BuildFinalMP4(tl, media.RenderOptions{
		Width: 640, Height: 360, FPS: 15, OutputMP4: dir + "/tutorial.mp4",
	}); err != nil {
		t.Fatalf("audio-only render should still succeed: %v", err)
	}
}
