package media

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/timeline"
)

func runFFmpeg(args ...string) (string, error) {
	out, err := exec.Command(FFmpegPath(), args...).CombinedOutput()
	return string(out), err
}

func dirOf(p string) string { return filepath.Dir(p) }

type CinematicOptions struct {
	Width         int
	Height        int
	FPS           int
	SceneVideo    map[string]string
	WorkDir       string
	OutputMP4     string
	NoZoom        bool
	DebugCuesPath string
}

func RenderCinematic(ft *timeline.FinalTimeline, opts CinematicOptions) error {
	if ft == nil || len(ft.Segments) == 0 {
		return fmt.Errorf("no final timeline segments; run record first")
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
	if len(ft.SceneClips) > 0 {
		if missing := missingFinalScenes(ft, opts.SceneVideo); len(missing) > 0 {
			return fmt.Errorf("missing raw video for scene(s) %s; run `autodoc record` (or --retake) first (refusing solid-color fallback)", strings.Join(missing, ", "))
		}
	}
	if err := os.MkdirAll(dirOf(opts.OutputMP4), 0o755); err != nil {
		return err
	}
	rawDur := map[string]float64{}
	for id, p := range opts.SceneVideo {
		rawDur[id] = probeDuration(p)
	}
	sceneIdx := map[string]int{}
	var inputs []string
	for i, sc := range ft.SceneClips {
		sceneIdx[sc.SceneID] = i
		inputs = append(inputs, "-i", opts.SceneVideo[sc.SceneID])
	}
	var parts []string
	sceneOuts := make([]string, len(ft.SceneClips))
	segCount := map[string]int{}
	for _, sg := range ft.Segments {
		si, ok := sceneIdx[sg.SceneID]
		if !ok {
			continue
		}
		n := segCount[sg.SceneID]
		segCount[sg.SceneID]++
		vn := fmt.Sprintf("[sg%d_%d]", si, n)
		chain := fmt.Sprintf("[%d:v]%s%s", si, segmentVideoChain(sg, rawDur[sg.SceneID], w, h, !opts.NoZoom), vn)
		parts = append(parts, chain)
		sceneOuts[si] += vn
	}
	for i, sc := range ft.SceneClips {
		n := segCount[sc.SceneID]
		if n == 0 {
			return fmt.Errorf("scene %s has no segments", sc.SceneID)
		}
		parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[vscene%d]", sceneOuts[i], n, i))
	}
	var vcat strings.Builder
	for i := range ft.SceneClips {
		fmt.Fprintf(&vcat, "[vscene%d]", i)
	}
	parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0,fps=%d,format=yuv420p[vout]", vcat.String(), len(ft.SceneClips), fps))
	nVideo := len(ft.SceneClips)
	var audioInputs []string
	var filterParts []string
	var amixInputs []string
	for i, sp := range ft.Speeches {
		audioInputs = append(audioInputs, "-i", sp.WavPath)
		delayMs := int(sp.StartS*1000 + 0.5)
		filterParts = append(filterParts, fmt.Sprintf("[%d:a]aformat=sample_fmts=fltp:channel_layouts=stereo,adelay=%d:all=1,apad,atrim=0:%.3f[a%d]", nVideo+i, delayMs, ft.TotalS+0.5, i))
		amixInputs = append(amixInputs, fmt.Sprintf("[a%d]", i))
	}
	args := []string{"-y"}
	args = append(args, inputs...)
	args = append(args, audioInputs...)
	filter := strings.Join(parts, ";")
	if len(ft.Speeches) > 0 {
		filter += ";" + strings.Join(filterParts, ";") + ";" + strings.Join(amixInputs, "") + fmt.Sprintf("amix=inputs=%d:normalize=0:duration=longest:dropout_transition=0[aout]", len(ft.Speeches))
	} else {
		// Speechless timeline: synthesize silence as a filter source
		// (anullsrc takes no input) trimmed to the video length.
		filter += fmt.Sprintf(";anullsrc=r=44100:cl=stereo,atrim=0:%.3f[aout]", ft.TotalS+0.5)
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
	out, err := runFFmpeg(args...)
	if err != nil {
		return fmt.Errorf("ffmpeg cinematic render: %w\n%s", err, tailLines([]byte(out), 20))
	}
	if opts.DebugCuesPath != "" {
		_ = WriteCuesDebug(ft, opts.DebugCuesPath)
	}
	return nil
}

func segmentVideoChain(sg timeline.AVSegment, rawDur float64, w, h int, zoomOK bool) string {
	vs, ve := sg.VideoStartS, sg.VideoEndS
	if vs < 0 {
		vs = 0
	}
	if rawDur > 0 && vs > rawDur-0.05 {
		vs = rawDur - 0.05
		if vs < 0 {
			vs = 0
		}
	}
	if ve <= vs+0.05 {
		ve = vs + 0.1
	}
	if rawDur > 0 && ve > rawDur {
		ve = rawDur
	}
	var b strings.Builder
	fmt.Fprintf(&b, "trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS", vs, ve)
	if sg.Speed > 1.01 {
		fmt.Fprintf(&b, ",setpts=(PTS-STARTPTS)/%.4f", sg.Speed)
	}
	// Target outlines (explicit focus / result emphasis) compose in source
	// space BEFORE the zoom crop so they hug the target through the move.
	if sg.Outline && sg.NormBBox != nil {
		b.WriteString("," + outlineBox(sg, 4))
	}
	if sg.ResultFlash {
		if sg.NormBBox != nil {
			b.WriteString("," + outlineBox(sg, 2))
		}
		b.WriteString(",eq=brightness=0.05")
	}
	if zoomOK && sg.Zoom > 1.01 && sg.NormBBox != nil {
		if z := zoomChain(sg, w, h); z != "" {
			b.WriteString("," + z)
		}
	}
	// Keyboard pill stays fixed in output space (after zoom): bottom-center
	// safe area above the subtitle band.
	if sg.Keyboard {
		b.WriteString("," + keyboardPill(sg.KeyLabel))
	}
	fmt.Fprintf(&b, ",tpad=stop_mode=clone:stop_duration=%.3f,trim=end=%.3f,setpts=PTS-STARTPTS,setsar=1", sg.DurS+0.1, sg.DurS)
	return b.String()
}

// outlineBox draws a discreet white target outline in source-space
// fractions (pre-zoom). Single calm emphasis, never continuous pulse.
func outlineBox(sg timeline.AVSegment, t int) string {
	nb := sg.NormBBox
	return fmt.Sprintf("drawbox=x='%.4f*iw':y='%.4f*ih':w='%.4f*iw':h='%.4f*ih':color=white@0.9:t=%d",
		nb.X, nb.Y, nb.Width, nb.Height, t)
}

// keyboardPill renders the keyboard shortcut overlay ("ENTER"): dark
// pill + bold label, bottom-center above subtitles. Solid border keeps
// the pill defined on both dark and light backdrops. NOTE: drawbox x/y
// expressions accept iw/ih (not bare w/h); drawtext accepts w/h.
func keyboardPill(label string) string {
	if label == "" {
		label = "KEY"
	}
	esc := strings.NewReplacer(`\`, `\\`, `:`, `\:`, `'`, `\'`, `%`, `\%`).Replace(label)
	return "drawbox=x='(iw-240)/2':y='ih-150':w=240:h=64:color=black@0.78:t=fill," +
		"drawbox=x='(iw-240)/2':y='ih-150':w=240:h=64:color=white@0.9:t=3," +
		"drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf:text='" + esc + "':fontsize=30:fontcolor=white:x='(w-text_w)/2':y='h-150+(64-text_h)/2'"
}

// ZoomChain returns the FFmpeg video filter chain for zooming into a segment.
func ZoomChain(sg timeline.AVSegment, w, h int) string {
	return zoomChain(sg, w, h)
}

func zoomChain(sg timeline.AVSegment, w, h int) string {
	D := sg.DurS
	if D <= 0.15 {
		return ""
	}
	T := 0.3
	if D/3 < T {
		T = D / 3
	}
	cx := sg.NormBBox.X*float64(w) + sg.NormBBox.Width*float64(w)/2
	cy := sg.NormBBox.Y*float64(h) + sg.NormBBox.Height*float64(h)/2

	var z string
	switch sg.CameraTransition {
	case "zoom_stay":
		z = fmt.Sprintf("%.4f", sg.Zoom)
	case "zoom_in":
		z = fmt.Sprintf("(1+(%.4f-1)*if(lt(t,%.3f),pow(t/%.3f\\,2)*(3-2*t/%.3f)\\,1))",
			sg.Zoom, T, T, T)
	case "zoom_out":
		z = fmt.Sprintf("(1+(%.4f-1)*if(gt(t\\,%.3f)\\,pow((%.3f-t)/%.3f\\,2)*(3-2*(%.3f-t)/%.3f)\\,1))",
			sg.Zoom, D-T, D, T, D, T)
	case "zoom_isolated", "":
		fallthrough
	default:
		z = fmt.Sprintf("(1+(%.4f-1)*if(lt(t,%.3f),pow(t/%.3f\\,2)*(3-2*t/%.3f)\\,if(gt(t\\,%.3f)\\,pow((%.3f-t)/%.3f\\,2)*(3-2*(%.3f-t)/%.3f)\\,1)))",
			sg.Zoom, T, T, T, D-T, D, T, D, T)
	}
	cw := fmt.Sprintf("iw/%s", z)
	ch := fmt.Sprintf("ih/%s", z)
	x := fmt.Sprintf("max(0\\,min(iw-(%s)\\,%.1f-(%s)/2))", cw, cx, cw)
	y := fmt.Sprintf("max(0\\,min(ih-(%s)\\,%.1f-(%s)/2))", ch, cy, ch)
	return fmt.Sprintf("crop=w='%s':h='%s':x='%s':y='%s',scale=%d:%d", cw, ch, x, y, w, h)
}

func missingFinalScenes(ft *timeline.FinalTimeline, sceneVideo map[string]string) []string {
	var missing []string
	for _, c := range ft.SceneClips {
		if _, ok := sceneVideo[c.SceneID]; !ok {
			missing = append(missing, c.SceneID)
		}
	}
	return missing
}

func WriteCuesDebug(ft *timeline.FinalTimeline, path string) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ft, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
