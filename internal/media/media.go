package media

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/timeline"
)

type Probe struct {
	Width    int
	Height   int
	Duration float64
	HasVideo bool
	HasAudio bool
	VCodec   string
	ACodec   string
}

func FFmpegPath() string {
	if p := os.Getenv("AUTODOC_FFMPEG"); p != "" {
		return p
	}
	return "ffmpeg"
}

func FFprobePath() string {
	if p := os.Getenv("AUTODOC_FFPROBE"); p != "" {
		return p
	}
	return "ffprobe"
}

func CheckFFmpeg() (string, error) {
	out, err := exec.Command(FFmpegPath(), "-version").Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg not found (%s): %w", FFmpegPath(), err)
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(first), nil
}

func ProbeFile(path string) (*Probe, error) {
	args := []string{"-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path}
	out, err := exec.Command(FFprobePath(), args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	var raw struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	p := &Probe{}
	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			p.HasVideo = true
			p.VCodec = s.CodecName
			p.Width, p.Height = s.Width, s.Height
		case "audio":
			p.HasAudio = true
			p.ACodec = s.CodecName
		}
	}
	if raw.Format.Duration != "" {
		p.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	}
	return p, nil
}

func WavDuration(path string) (float64, error) {
	p, err := ProbeFile(path)
	if err != nil {
		return 0, err
	}
	return p.Duration, nil
}

type RenderOptions struct {
	Width      int
	Height     int
	FPS        int
	SceneVideo map[string]string
	WorkDir    string
	OutputMP4  string
	Overwrite  bool
}

func ConcatAudio(wavs []string, out string) error {
	if len(wavs) == 0 {
		return fmt.Errorf("no wavs to concat")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if len(wavs) == 1 {
		return copyFile(wavs[0], out)
	}
	listFile := out + ".list.txt"
	var sb strings.Builder
	for _, w := range wavs {
		abs, _ := filepath.Abs(w)
		sb.WriteString("file '" + strings.ReplaceAll(abs, "'", "'\\''") + "'\n")
	}
	if err := os.WriteFile(listFile, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	args := []string{"-y", "-f", "concat", "-safe", "0", "-i", listFile, "-c", "copy", out}
	if out2, err := exec.Command(FFmpegPath(), args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg concat: %w\n%s", err, tailLines(out2, 20))
	}
	return nil
}

func BuildFinalMP4(tl *timeline.Timeline, opts RenderOptions) error {
	if tl == nil {
		return fmt.Errorf("no timeline provided; run tts/record first")
	}
	// Refuse to silently render solid-color "black video": a missing raw
	// capture used to produce a plausible-looking MP4 of flat 0x1a1d29.
	// Failing here is cheaper than publishing a tutorial with no app frames.
	// (Timelines without scene clips — e.g. audio-only unit-test fixtures —
	// keep the legacy color fallback.)
	if len(tl.SceneClips) > 0 {
		if len(opts.SceneVideo) == 0 {
			return fmt.Errorf("no scene video found for %d timeline scene clip(s); run `autodoc record` first (refusing solid-color fallback)", len(tl.SceneClips))
		}
		if missing := missingScenes(tl, opts.SceneVideo); len(missing) > 0 {
			return fmt.Errorf("missing raw video for scene(s) %s; run `autodoc record` (or --retake) first (refusing solid-color fallback)", strings.Join(missing, ", "))
		}
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutputMP4), 0o755); err != nil {
		return err
	}
	w, h, fps := opts.Width, opts.Height, opts.FPS
	if w == 0 {
		w = 1280
	}
	if h == 0 {
		h = 720
	}
	if fps == 0 {
		fps = 30
	}
	var audioInputs []string
	var filterParts []string
	var amixInputs []string
	for i, seg := range tl.Segments {
		audioInputs = append(audioInputs, "-i", seg.WavPath)
		delayMs := int(seg.StartS*1000 + 0.5)
		filterParts = append(filterParts, fmt.Sprintf("[%d:a]aformat=sample_fmts=fltp:channel_layouts=stereo,adelay=%d:all=1,apad,atrim=0:%.3f[a%d]", i+1, delayMs, tl.TotalS+0.5, i))
		amixInputs = append(amixInputs, fmt.Sprintf("[a%d]", i))
	}
	nSeg := len(tl.Segments)
	var videoSrcArgs []string
	var videoInputs []string
	var videoFilter string
	nVideo := 0
	if len(opts.SceneVideo) == 0 {
		videoSrcArgs = []string{"-f", "lavfi", "-i", fmt.Sprintf("color=c=0x1a1d29:s=%dx%d:r=%d:d=%.3f", w, h, fps, tl.TotalS+0.5)}
		videoFilter = "[0:v]format=yuv420p[vout]"
		nVideo = 1
	} else {
		ordered := sceneOrder(tl, opts.SceneVideo)
		padColor := "0x1a1d29"
		scaleChain := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=%s,setsar=1,fps=%d", w, h, w, h, padColor, fps)
		var vparts []string
		var concatIns []string
		for i, sc := range ordered {
			videoInputs = append(videoInputs, "-i", sc.path)
			dur := sc.dur
			if dur <= 0 {
				dur = 0.5
			}
			rawDur := probeDuration(sc.path)
			chain := fmt.Sprintf("[%d:v]%s", i, scaleChain)
			switch {
			case rawDur <= 0:
				chain += fmt.Sprintf(",trim=0:%.3f,setpts=PTS-STARTPTS", dur)
			case rawDur < dur:
				chain += fmt.Sprintf(",trim=0:%.3f,setpts=PTS-STARTPTS,tpad=stop_mode=clone:stop_duration=%.3f", rawDur, dur-rawDur+0.05)
			default:
				chain += fmt.Sprintf(",trim=0:%.3f,setpts=PTS-STARTPTS", dur)
			}
			chain += fmt.Sprintf("[v%d]", i)
			vparts = append(vparts, chain)
			concatIns = append(concatIns, fmt.Sprintf("[v%d]", i))
		}
		videoFilter = strings.Join(vparts, ";") + ";" + strings.Join(concatIns, "") + fmt.Sprintf("concat=n=%d:v=1:a=0,format=yuv420p[vout]", len(ordered))
		nVideo = len(ordered)
		videoSrcArgs = videoInputs
	}
	args := []string{"-y"}
	args = append(args, videoSrcArgs...)
	args = append(args, audioInputs...)
	var filter string
	if nSeg > 0 {
		shifted := make([]string, 0, len(filterParts))
		for i, fp := range filterParts {
			shifted = append(shifted, strings.Replace(fp, fmt.Sprintf("[%d:a]", i+1), fmt.Sprintf("[%d:a]", nVideo+i), 1))
		}
		audioChain := strings.Join(shifted, ";") + ";" + strings.Join(amixInputs, "") + fmt.Sprintf("amix=inputs=%d:normalize=0:duration=longest:dropout_transition=0[aout]", nSeg)
		filter = videoFilter + ";" + audioChain
	} else {
		filter = videoFilter
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo")
		filter += ";[1:a]anullsrc[aout]"
		_ = nSeg
	}
	args = append(args,
		"-filter_complex", filter,
		"-map", "[vout]", "-map", "[aout]",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-r", fmt.Sprint(fps),
		"-c:a", "aac", "-b:a", "128k", "-ar", "44100",
		"-shortest",
		"-movflags", "+faststart",
		opts.OutputMP4,
	)
	out, err := exec.Command(FFmpegPath(), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg render: %w\n%s", err, tailLines(out, 20))
	}
	return nil
}

func RenderSceneClip(srcMP4, dstMP4 string, startS, durS float64, width, height int) error {
	if err := os.MkdirAll(filepath.Dir(dstMP4), 0o755); err != nil {
		return err
	}
	scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,format=yuv420p", width, height, width, height)
	args := []string{"-y", "-ss", fmt.Sprintf("%.3f", startS), "-t", fmt.Sprintf("%.3f", durS),
		"-i", srcMP4, "-vf", scale, "-c:v", "libx264", "-preset", "veryfast", "-an", dstMP4}
	out, err := exec.Command(FFmpegPath(), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg scene clip: %w\n%s", err, tailLines(out, 20))
	}
	return nil
}

func MakeThumbnail(srcMP4, dstPNG string, atS float64, width int) error {
	if err := os.MkdirAll(filepath.Dir(dstPNG), 0o755); err != nil {
		return err
	}
	args := []string{"-y", "-ss", fmt.Sprintf("%.3f", atS), "-i", srcMP4, "-frames:v", "1"}
	if width > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:-2", width))
	}
	args = append(args, dstPNG)
	out, err := exec.Command(FFmpegPath(), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w\n%s", err, tailLines(out, 20))
	}
	return nil
}

// tailLines keeps the last n lines of verbose backend output. ffmpeg logs
// are longest at the head (configuration banners); the failure cause is at
// the tail, so agent-facing errors carry the tail only.
func tailLines(out []byte, n int) string {
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func WriteSRT(tl *timeline.Timeline, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for i, s := range tl.Segments {
		fmt.Fprintf(w, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(s.StartS), srtTime(s.EndS), s.Text)
	}
	return w.Flush()
}

func WriteVTT(tl *timeline.Timeline, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "WEBVTT")
	fmt.Fprintln(w)
	for _, s := range tl.Segments {
		fmt.Fprintf(w, "%s --> %s\n%s\n\n", vttTime(s.StartS), vttTime(s.EndS), s.Text)
	}
	return w.Flush()
}

func srtTime(s float64) string {
	ms := int(s*1000 + 0.5)
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	sec := (ms % 60000) / 1000
	rem := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, sec, rem)
}

func vttTime(s float64) string {
	ms := int(s*1000 + 0.5)
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	sec := (ms % 60000) / 1000
	rem := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, sec, rem)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

type sceneInput struct {
	path string
	dur  float64
}

func missingScenes(tl *timeline.Timeline, sceneVideo map[string]string) []string {
	var missing []string
	seen := map[string]bool{}
	for _, c := range tl.SceneClips {
		if seen[c.SceneID] {
			continue
		}
		seen[c.SceneID] = true
		if _, ok := sceneVideo[c.SceneID]; !ok {
			missing = append(missing, c.SceneID)
		}
	}
	return missing
}

func sceneOrder(tl *timeline.Timeline, sceneVideo map[string]string) []sceneInput {
	ordered := []sceneInput{}
	seen := map[string]bool{}
	for _, c := range tl.SceneClips {
		if p, ok := sceneVideo[c.SceneID]; ok && !seen[c.SceneID] {
			ordered = append(ordered, sceneInput{path: p, dur: c.Duration})
			seen[c.SceneID] = true
		}
	}
	for id, p := range sceneVideo {
		if !seen[id] {
			ordered = append(ordered, sceneInput{path: p, dur: probeDuration(p)})
			seen[id] = true
		}
	}
	return ordered
}

func probeDuration(path string) float64 {
	p, err := ProbeFile(path)
	if err != nil {
		return 0
	}
	return p.Duration
}
