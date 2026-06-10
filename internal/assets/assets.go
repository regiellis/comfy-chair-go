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

// Filter captures a listing query. Zero values mean "no constraint".
// It drives both the grid views and bulk actions, so "act on everything
// matching the current view" is exact.
type Filter struct {
	Env    string    `json:"env"`
	Type   string    `json:"type"`
	Query  string    `json:"q"`
	Folder string    `json:"folder"` // relative dir inside the type root; "" = everywhere
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Sort   string    `json:"sort"`
}

// scan walks the asset directories of the requested environments and types.
// envName/assetType may be empty to mean "all".
func scan(envs []Env, envName, assetType string) []Asset {
	return scanFilter(envs, Filter{Env: envName, Type: assetType})
}

// scanFilter walks the matching asset directories and applies the filter.
func scanFilter(envs []Env, f Filter) []Asset {
	var out []Asset
	folder := strings.Trim(filepath.ToSlash(filepath.Clean(f.Folder)), "/")
	if folder == "." {
		folder = ""
	}
	query := strings.ToLower(strings.TrimSpace(f.Query))
	for _, e := range envs {
		if f.Env != "" && e.Name != f.Env {
			continue
		}
		for t, sub := range typeRoots {
			if f.Type != "" && t != f.Type {
				continue
			}
			root := filepath.Join(e.Path, sub)
			if folder != "" {
				root = filepath.Join(root, filepath.FromSlash(folder))
			}
			for _, a := range scanRoot(e.Name, t, root) {
				if folder != "" {
					a.Path = folder + "/" + a.Path
				}
				if query != "" && !strings.Contains(strings.ToLower(a.Path), query) {
					continue
				}
				if !f.From.IsZero() && a.ModTime.Before(f.From) {
					continue
				}
				if !f.To.IsZero() && a.ModTime.After(f.To) {
					continue
				}
				out = append(out, a)
			}
		}
	}
	sortAssets(out, f.Sort)
	return out
}

// subfolders lists the immediate child directories of the filter's folder
// across the matching envs/types (union, sorted, trash excluded).
func subfolders(envs []Env, f Filter) []string {
	seen := map[string]bool{}
	folder := strings.Trim(filepath.ToSlash(filepath.Clean(f.Folder)), "/")
	if folder == "." {
		folder = ""
	}
	for _, e := range envs {
		if f.Env != "" && e.Name != f.Env {
			continue
		}
		for t, sub := range typeRoots {
			if f.Type != "" && t != f.Type {
				continue
			}
			dir := filepath.Join(e.Path, sub, filepath.FromSlash(folder))
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, ent := range entries {
				if ent.IsDir() && !strings.HasPrefix(ent.Name(), ".") {
					seen[ent.Name()] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// GroupLabel returns the day-group heading for a timestamp.
func GroupLabel(t time.Time, now time.Time) string {
	y, m, d := t.Local().Date()
	ny, nm, nd := now.Local().Date()
	switch {
	case y == ny && m == nm && d == nd:
		return "Today"
	case t.Local().AddDate(0, 0, 1).Day() == nd && t.Local().AddDate(0, 0, 1).Month() == nm && t.Local().AddDate(0, 0, 1).Year() == ny:
		return "Yesterday"
	default:
		return t.Local().Format("Mon, Jan 2 2006")
	}
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

// validName rejects names that could escape a directory or hide files.
func validName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid name %q", name)
	}
	return nil
}

// makeFolder creates a directory inside an env/type root.
func makeFolder(envs []Env, envName, assetType, folder string) error {
	abs, err := resolveAssetPath(envs, envName, assetType, folder)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0755)
}

// renameAsset renames a file within its directory.
func renameAsset(envs []Env, envName, assetType, relPath, newName string) error {
	if err := validName(newName); err != nil {
		return err
	}
	src, err := resolveAssetPath(envs, envName, assetType, relPath)
	if err != nil {
		return err
	}
	dst := filepath.Join(filepath.Dir(src), newName)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", newName)
	}
	return os.Rename(src, dst)
}

// moveToFolder moves an asset to another folder within the same env/type root.
func moveToFolder(envs []Env, envName, assetType, relPath, dstFolder string) error {
	src, err := resolveAssetPath(envs, envName, assetType, relPath)
	if err != nil {
		return err
	}
	dstRel := filepath.Base(relPath)
	if dstFolder != "" {
		dstRel = strings.Trim(filepath.ToSlash(dstFolder), "/") + "/" + dstRel
	}
	dst, err := resolveAssetPath(envs, envName, assetType, dstRel)
	if err != nil {
		return err
	}
	if src == dst {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		// Cross-device fallback.
		if err := copyFile(src, dst); err != nil {
			return err
		}
		return os.Remove(src)
	}
	return nil
}

// folderRef identifies a folder within an env/type root.
type folderRef struct {
	Env    string `json:"env"`
	Type   string `json:"type"`
	Folder string `json:"folder"`
}

// findEmptyFolders returns folders under the matching roots that contain no
// files at any depth, deepest first so removal can proceed bottom-up. The
// .trash dirs are ignored entirely.
func findEmptyFolders(envs []Env, envName, assetType string) []folderRef {
	var out []folderRef
	for _, e := range envs {
		if envName != "" && e.Name != envName {
			continue
		}
		for t, sub := range typeRoots {
			if assetType != "" && t != assetType {
				continue
			}
			root := filepath.Join(e.Path, sub)
			hasFile := map[string]bool{}
			var dirs []string
			_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					if info.Name() == trashDirName || (strings.HasPrefix(info.Name(), ".") && p != root) {
						return filepath.SkipDir
					}
					if p != root {
						dirs = append(dirs, p)
					}
					return nil
				}
				for d := filepath.Dir(p); strings.HasPrefix(d, root); d = filepath.Dir(d) {
					if hasFile[d] {
						break
					}
					hasFile[d] = true
					if d == root {
						break
					}
				}
				return nil
			})
			for _, d := range dirs {
				if !hasFile[d] {
					rel, err := filepath.Rel(root, d)
					if err == nil {
						out = append(out, folderRef{Env: e.Name, Type: t, Folder: filepath.ToSlash(rel)})
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i].Folder) > len(out[j].Folder) })
	return out
}

// removeEmptyFolders deletes every empty folder in scope. os.Remove refuses
// non-empty directories, so a race with new files cannot destroy content.
func removeEmptyFolders(envs []Env, envName, assetType string) []folderRef {
	var removed []folderRef
	for _, fr := range findEmptyFolders(envs, envName, assetType) {
		abs, err := resolveAssetPath(envs, fr.Env, fr.Type, fr.Folder)
		if err != nil {
			continue
		}
		if os.Remove(abs) == nil {
			removed = append(removed, fr)
		}
	}
	return removed
}

// uncollide returns dst, or a timestamp-suffixed variant when dst exists.
func uncollide(dst string) string {
	if _, err := os.Stat(dst); err != nil {
		return dst
	}
	ext := filepath.Ext(dst)
	return strings.TrimSuffix(dst, ext) + time.Now().Format("_20060102-150405") + ext
}

// deleteFolder removes a folder inside an env/type root. With keepContents,
// the folder's entries move up into its parent first and the empty folder is
// removed; otherwise the entire folder moves into .trash (recoverable).
func deleteFolder(envs []Env, envName, assetType, folder string, keepContents bool) error {
	rel := strings.Trim(filepath.ToSlash(filepath.Clean(folder)), "/")
	if rel == "" || rel == "." {
		return fmt.Errorf("refusing to delete the asset root")
	}
	abs, err := resolveAssetPath(envs, envName, assetType, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a folder", rel)
	}

	if keepContents {
		parent := filepath.Dir(abs)
		entries, err := os.ReadDir(abs)
		if err != nil {
			return err
		}
		for _, ent := range entries {
			dst := uncollide(filepath.Join(parent, ent.Name()))
			if err := os.Rename(filepath.Join(abs, ent.Name()), dst); err != nil {
				return err
			}
		}
		return os.Remove(abs)
	}

	root, err := resolveRoot(envs, envName, assetType)
	if err != nil {
		return err
	}
	dst := uncollide(filepath.Join(root, trashDirName, filepath.FromSlash(rel)))
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Rename(abs, dst)
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

// readFileCapped reads at most capBytes of a file.
func readFileCapped(path string, capBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, capBytes))
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
