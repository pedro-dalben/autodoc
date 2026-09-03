package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Manifest struct {
	Version        int               `json:"version"`
	AutodocVersion string            `json:"autodoc_version"`
	UpdatedAt      string            `json:"updated_at"`
	Items          []OwnedItem       `json:"items"`
	Backups        map[string]string `json:"backups,omitempty"`
}

type OwnedItem struct {
	Kind    string `json:"kind"`
	Harness string `json:"harness,omitempty"`
	Path    string `json:"path"`
	Marker  string `json:"marker,omitempty"`
	Note    string `json:"note,omitempty"`
}

func ManifestPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "installations.json"), nil
}

func ConfigDir() (string, error) {
	if v := os.Getenv("AUTODOC_CONFIG_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "autodoc"), nil
	}
	return filepath.Join(home, ".config", "autodoc"), nil
}

func DataDir() (string, error) {
	if v := os.Getenv("AUTODOC_DATA_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "autodoc"), nil
	}
	return filepath.Join(home, ".local", "share", "autodoc"), nil
}

func SkillDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "skills"), nil
}

func ProfilesDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profiles"), nil
}

func CacheDir() (string, error) {
	if v := os.Getenv("AUTODOC_CACHE_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "autodoc"), nil
	}
	return filepath.Join(home, ".cache", "autodoc"), nil
}

func LoadManifest() (*Manifest, error) {
	p, err := ManifestPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Version: 1, Items: nil, Backups: map[string]string{}}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", p, err)
	}
	if m.Backups == nil {
		m.Backups = map[string]string{}
	}
	return &m, nil
}

func (m *Manifest) Save(autodocVersion string) error {
	p, err := ManifestPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	m.Version = 1
	m.AutodocVersion = autodocVersion
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

func (m *Manifest) Owns(path string) bool {
	for _, it := range m.Items {
		if it.Path == path {
			return true
		}
	}
	return false
}

func (m *Manifest) Upsert(item OwnedItem) {
	for i := range m.Items {
		if m.Items[i].Path == item.Path && m.Items[i].Kind == item.Kind {
			m.Items[i] = item
			return
		}
	}
	m.Items = append(m.Items, item)
}

func (m *Manifest) RemoveByPath(path string) {
	out := m.Items[:0]
	for _, it := range m.Items {
		if it.Path != path {
			out = append(out, it)
		}
	}
	m.Items = out
}

func (m *Manifest) RemoveByHarness(harness string) []OwnedItem {
	var removed []OwnedItem
	out := m.Items[:0]
	for _, it := range m.Items {
		if it.Harness == harness {
			removed = append(removed, it)
		} else {
			out = append(out, it)
		}
	}
	m.Items = out
	return removed
}
