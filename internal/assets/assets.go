// Package assets implements the comfy-chair asset manager: a local web UI
// for browsing and managing images, videos, audio, and workflows across all
// registered ComfyUI environments.
package assets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Env is a ComfyUI installation the asset manager operates on.
type Env struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Asset type roots, relative to an environment's ComfyUI directory.
var typeRoots = map[string]string{
	"output":   "output",
	"input":    "input",
	"workflow": filepath.Join("user", "default", "workflows"),
}

// kindForExt maps file extensions to an asset kind the frontend understands.
var kindForExt = map[string]string{
	".png": "image", ".jpg": "image", ".jpeg": "image", ".webp": "image",
	".gif": "image", ".bmp": "image",
	".mp4": "video", ".webm": "video", ".mov": "video", ".mkv": "video", ".avi": "video",
	".wav": "audio", ".mp3": "audio", ".flac": "audio", ".ogg": "audio",
	".json": "workflow",
}

// Asset is a single file in an environment's asset directories.
type Asset struct {
	Env     string    `json:"env"`
	Type    string    `json:"type"` // output | input | workflow
	Path    string    `json:"path"` // relative to the type root, slash-separated
	Name    string    `json:"name"`
	Kind    string    `json:"kind"` // image | video | audio | workflow
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
}

const trashDirName = ".trash"

// resolveRoot returns the absolute directory for an env/type pair.
func resolveRoot(envs []Env, envName, assetType string) (string, error) {
	sub, ok := typeRoots[assetType]
	if !ok {
		return "", fmt.Errorf("unknown asset type %q", assetType)
	}
	for _, e := range envs {
		if e.Name == envName {
			return filepath.Join(e.Path, sub), nil
		}
	}
	return "", fmt.Errorf("unknown environment %q", envName)
}

// resolveAssetPath validates a client-supplied relative path and returns the
// absolute path, guaranteed to live inside the env/type root.
func resolveAssetPath(envs []Env, envName, assetType, relPath string) (string, error) {
	root, err := resolveRoot(envs, envName, assetType)
	if err != nil {
		return "", err
	}
	rel := filepath.Clean(filepath.FromSlash(relPath))
	if rel == "." || rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid path %q", relPath)
	}
	abs := filepath.Join(root, rel)
	if !strings.HasPrefix(abs, filepath.Clean(root)+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the asset root", relPath)
	}
	return abs, nil
}

// scan walks the asset directories of the requested environments and types.
// envName/assetType may be empty to mean "all".
func scan(envs []Env, envName, assetType string) []Asset {
	var out []Asset
	for _, e := range envs {
		if envName != "" && e.Name != envName {
			continue
		}
		for t, sub := range typeRoots {
			if assetType != "" && t != assetType {
				continue
			}
			root := filepath.Join(e.Path, sub)
			out = append(out, scanRoot(e.Name, t, root)...)
		}
	}
	return out
}

func scanRoot(envName, assetType, root string) []Asset {
	var out []Asset
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if info.IsDir() {
			if info.Name() == trashDirName || strings.HasPrefix(info.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		kind, ok := kindForExt[strings.ToLower(filepath.Ext(path))]
		if !ok {
			return nil
		}
		// Workflow roots only contain workflows; media roots ignore stray json.
		if assetType == "workflow" && kind != "workflow" {
			return nil
		}
		if assetType != "workflow" && kind == "workflow" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		out = append(out, Asset{
			Env:     envName,
			Type:    assetType,
			Path:    filepath.ToSlash(rel),
			Name:    info.Name(),
			Kind:    kind,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		return nil
	})
	return out
}

func sortAssets(items []Asset, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	case "size":
		sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	default: // date, newest first
		sort.Slice(items, func(i, j int) bool { return items[i].ModTime.After(items[j].ModTime) })
	}
}

// moveToTrash moves an asset into the .trash directory of its type root,
// preserving the relative path. A timestamp suffix avoids collisions.
func moveToTrash(envs []Env, envName, assetType, relPath string) error {
	abs, err := resolveAssetPath(envs, envName, assetType, relPath)
	if err != nil {
		return err
	}
	root, _ := resolveRoot(envs, envName, assetType)
	dst := filepath.Join(root, trashDirName, filepath.FromSlash(relPath))
	if _, err := os.Stat(dst); err == nil {
		ext := filepath.Ext(dst)
		dst = strings.TrimSuffix(dst, ext) + time.Now().Format("_20060102-150405") + ext
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := os.Rename(abs, dst); err != nil {
		return err
	}
	return nil
}

// transfer copies (or moves) an asset to the same type directory of another
// environment, preserving the relative path.
func transfer(envs []Env, envName, assetType, relPath, dstEnv string, move bool) error {
	src, err := resolveAssetPath(envs, envName, assetType, relPath)
	if err != nil {
		return err
	}
	dst, err := resolveAssetPath(envs, dstEnv, assetType, relPath)
	if err != nil {
		return err
	}
	if src == dst {
		return fmt.Errorf("source and destination are the same file")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	if move {
		return os.Remove(src)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// forEachTrashDir visits the .trash directory of every matching env/type
// pair. Empty selectors mean "all".
func forEachTrashDir(envs []Env, envName, assetType string, fn func(trash string)) {
	for _, e := range envs {
		if envName != "" && e.Name != envName {
			continue
		}
		for t, sub := range typeRoots {
			if assetType != "" && t != assetType {
				continue
			}
			fn(filepath.Join(e.Path, sub, trashDirName))
		}
	}
}

func trashContents(trash string) (files int, bytes int64) {
	_ = filepath.Walk(trash, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files++
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes
}

// TrashStats reports how many files (and bytes) sit in the matching .trash
// directories without deleting anything.
func TrashStats(envs []Env, envName, assetType string) (files int, bytes int64) {
	forEachTrashDir(envs, envName, assetType, func(trash string) {
		f, b := trashContents(trash)
		files += f
		bytes += b
	})
	return files, bytes
}

// EmptyTrash permanently removes the .trash directories of the matching
// environments/types and reports what was freed.
func EmptyTrash(envs []Env, envName, assetType string) (files int, bytes int64) {
	forEachTrashDir(envs, envName, assetType, func(trash string) {
		f, b := trashContents(trash)
		files += f
		bytes += b
		_ = os.RemoveAll(trash)
	})
	return files, bytes
}

// saveWorkflowJSON writes extracted workflow JSON into the destination
// environment's workflows directory.
func saveWorkflowJSON(envs []Env, dstEnv, name string, data []byte) (string, error) {
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		name += ".json"
	}
	dst, err := resolveAssetPath(envs, dstEnv, "workflow", name)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dst); err == nil {
		ext := filepath.Ext(dst)
		dst = strings.TrimSuffix(dst, ext) + time.Now().Format("_20060102-150405") + ext
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return "", err
	}
	return filepath.Base(dst), nil
}
