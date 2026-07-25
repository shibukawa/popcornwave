package pwcli

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func preparePublicAssets(root string) error {
	publicRoot := filepath.Join(root, "public")
	if info, err := os.Lstat(filepath.Join(root, "public.go")); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("public.go is required; run pw init to create the public embed scaffold")
	}
	rootInfo, err := os.Lstat(publicRoot)
	if err != nil {
		return fmt.Errorf("public assets: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("public assets: public must be a regular directory")
	}

	encoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithEncoderConcurrency(1),
		zstd.WithEncoderCRC(false),
	)
	if err != nil {
		return fmt.Errorf("public assets: create zstd encoder: %w", err)
	}
	defer encoder.Close()

	eligible := make(map[string]bool)
	var sidecars []string
	err = filepath.WalkDir(publicRoot, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == publicRoot {
			return nil
		}
		info, err := os.Lstat(name)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("public assets: symbolic links are not allowed: %s", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("public assets: irregular file is not allowed: %s", name)
		}
		if strings.HasSuffix(name, ".zstd") {
			sidecars = append(sidecars, name)
			return nil
		}
		if entry.Name() == ".keep" || !publicAssetCompressible(name) {
			return nil
		}
		sidecar := name + ".zstd"
		eligible[sidecar] = true
		source, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		encoded := encoder.EncodeAll(source, nil)
		if err := writeScaffoldFile(sidecar, encoded); err != nil {
			return fmt.Errorf("public assets: write %s: %w", sidecar, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, sidecar := range sidecars {
		if !eligible[sidecar] {
			if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("public assets: remove stale %s: %w", sidecar, err)
			}
		}
	}
	return nil
}

func publicAssetCompressible(name string) bool {
	mediaType := strings.ToLower(mime.TypeByExtension(filepath.Ext(name)))
	if separator := strings.IndexByte(mediaType, ';'); separator >= 0 {
		mediaType = mediaType[:separator]
	}
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/javascript", "application/json", "application/manifest+json",
		"application/xml", "image/svg+xml":
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".css", ".js", ".mjs", ".json", ".map", ".txt", ".xml", ".svg", ".webmanifest":
		return true
	default:
		return false
	}
}
