package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Project ProjectConfig `toml:"project"`
	TTS     TTSConfig     `toml:"tts"`
	Media   MediaConfig   `toml:"media"`
	Browser BrowserConfig `toml:"browser"`
	Paths   PathsConfig   `toml:"paths"`
}

type ProjectConfig struct {
	Name     string `toml:"name"`
	Language string `toml:"language"`
}

type TTSConfig struct {
	Provider       string  `toml:"provider"`
	BaseURL        string  `toml:"base_url"`
	APIKeyEnv      string  `toml:"api_key_env"`
	Model          string  `toml:"model"`
	Voice          string  `toml:"voice"`
	Language       string  `toml:"language"`
	Speed          float64 `toml:"speed"`
	ResponseFormat string  `toml:"response_format"`
}

type MediaConfig struct {
	Resolution string `toml:"resolution"`
	FPS        int    `toml:"fps"`
	Codec      string `toml:"codec"`
}

type BrowserConfig struct {
	Profile      string `toml:"profile"`
	Headless     bool   `toml:"headless"`
	ViewportW    int    `toml:"viewport_width"`
	ViewportH    int    `toml:"viewport_height"`
	StorageState string `toml:"storage_state"`
}

type PathsConfig struct {
	Storyboard string `toml:"storyboard"`
	WorkDir    string `toml:"work_dir"`
	OutputDir  string `toml:"output_dir"`
}

func Default() *Config {
	return &Config{
		Project: ProjectConfig{Language: "pt-BR"},
		TTS: TTSConfig{
			Provider:       "openai-compatible",
			BaseURL:        "http://localhost:8880/v1",
			Model:          "kokoro",
			Language:       "pt-BR",
			Speed:          1.0,
			ResponseFormat: "wav",
		},
		Media:   MediaConfig{Resolution: "1920x1080", FPS: 30, Codec: "libx264"},
		Browser: BrowserConfig{Profile: "docs", Headless: true, ViewportW: 1280, ViewportH: 720},
		Paths:   PathsConfig{Storyboard: "storyboard.yml", WorkDir: ".autodoc/_work", OutputDir: "docs/autodoc"},
	}
}

func DefaultDisabledTTS() *Config {
	c := Default()
	c.TTS.Provider = "disabled"
	return c
}

func (c *Config) Validate() []error {
	var errs []error
	switch c.TTS.Provider {
	case "openai-compatible", "disabled":
	default:
		errs = append(errs, fmt.Errorf("tts.provider must be openai-compatible or disabled, got %q", c.TTS.Provider))
	}
	if c.TTS.Provider == "openai-compatible" {
		if c.TTS.BaseURL == "" {
			errs = append(errs, fmt.Errorf("tts.base_url is required for openai-compatible"))
		}
		if c.TTS.Model == "" {
			errs = append(errs, fmt.Errorf("tts.model is required for openai-compatible"))
		}
		if c.TTS.Speed <= 0 || c.TTS.Speed > 4 {
			errs = append(errs, fmt.Errorf("tts.speed must be in (0,4], got %v", c.TTS.Speed))
		}
	}
	return errs
}

func FindProjectRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "autodoc.toml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("autodoc.toml not found (run autodoc init first)")
		}
		dir = parent
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := Default()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
