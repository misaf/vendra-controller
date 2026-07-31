package renderer

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/misaf/vendra-controller/assets"
	"github.com/misaf/vendra-controller/internal/filesystem"
)

type Renderer struct{}

func (Renderer) Project(kind, destination string) error {
	root := "compose/" + kind
	return fs.WalkDir(assets.Files, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := assets.Files.ReadFile(path)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, root+"/")
		if err := filesystem.AtomicWrite(filepath.Join(destination, rel), data, 0o644); err != nil {
			return fmt.Errorf("render %s: %w", kind, err)
		}
		return nil
	})
}
