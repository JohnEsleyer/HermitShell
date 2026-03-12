package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
)

type Workspace struct {
	RootDir string
	OutDir  string
}

func New(root string) *Workspace {
	return &Workspace{RootDir: root, OutDir: filepath.Join(root, "out")}
}

func (w *Workspace) ListFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		files = append(files, e.Name())
	}
	return files, nil
}

func (w *Workspace) WriteFile(rel string, data []byte) error {
	full := filepath.Join(w.RootDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, fs.FileMode(0o644))
}

func (w *Workspace) ReadFile(rel string) ([]byte, error) {
	full := filepath.Join(w.RootDir, rel)
	return os.ReadFile(full)
}
