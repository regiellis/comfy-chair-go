package assets

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTestPNG writes a small PNG with ComfyUI-style tEXt chunks spliced in
// after IHDR (prompt + workflow), the same layout ComfyUI produces.
func writeTestPNG(t *testing.T, path string, withMeta bool) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range 4 {
		img.Set(i, i, color.RGBA{R: 255, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if withMeta {
		// IHDR ends at signature(8) + length(4) + type(4) + 13 + crc(4) = 33.
		var meta bytes.Buffer
		meta.Write(data[:33])
		for key, val := range map[string]string{
			"prompt":   `{"1": {"class_type": "TestNode"}}`,
			"workflow": `{"nodes": [{"id": 1, "type": "TestNode"}], "links": []}`,
		} {
			payload := append(append([]byte(key), 0), []byte(val)...)
			var chunk bytes.Buffer
			binary.Write(&chunk, binary.BigEndian, uint32(len(payload)))
			chunk.WriteString("tEXt")
			chunk.Write(payload)
			crc := crc32.NewIEEE()
			crc.Write([]byte("tEXt"))
			crc.Write(payload)
			binary.Write(&chunk, binary.BigEndian, crc.Sum32())
			meta.Write(chunk.Bytes())
		}
		meta.Write(data[33:])
		data = meta.Bytes()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// newTestEnvs builds two environments with a few assets each.
func newTestEnvs(t *testing.T) []Env {
	t.Helper()
	root := t.TempDir()
	envs := []Env{
		{Name: "lounge", Path: filepath.Join(root, "lounge")},
		{Name: "den", Path: filepath.Join(root, "den")},
	}
	writeTestPNG(t, filepath.Join(envs[0].Path, "output", "gen_001.png"), true)
	writeTestPNG(t, filepath.Join(envs[0].Path, "output", "sub", "gen_002.png"), false)
	writeTestPNG(t, filepath.Join(envs[0].Path, "input", "ref.png"), false)
	for _, p := range []string{
		filepath.Join(envs[0].Path, "user", "default", "workflows", "wf_a.json"),
		filepath.Join(envs[1].Path, "user", "default", "workflows", "wf_b.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"nodes": []}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(envs[1].Path, "output"), 0755); err != nil {
		t.Fatal(err)
	}
	return envs
}

func TestResolveAssetPathSafety(t *testing.T) {
	envs := newTestEnvs(t)
	for _, bad := range []string{"../escape.png", "/etc/passwd", "..", "a/../../x"} {
		if _, err := resolveAssetPath(envs, "lounge", "output", bad); err == nil {
			t.Errorf("path %q should have been rejected", bad)
		}
	}
	if _, err := resolveAssetPath(envs, "lounge", "output", "sub/gen_002.png"); err != nil {
		t.Errorf("valid path rejected: %v", err)
	}
	if _, err := resolveAssetPath(envs, "nope", "output", "x.png"); err == nil {
		t.Error("unknown env should be rejected")
	}
	if _, err := resolveAssetPath(envs, "lounge", "models", "x.png"); err == nil {
		t.Error("unknown type should be rejected")
	}
}

func TestScan(t *testing.T) {
	envs := newTestEnvs(t)
	all := scan(envs, "", "")
	if len(all) != 5 {
		t.Fatalf("expected 5 assets, got %d: %+v", len(all), all)
	}
	outputs := scan(envs, "lounge", "output")
	if len(outputs) != 2 {
		t.Fatalf("expected 2 lounge outputs, got %d", len(outputs))
	}
	wfs := scan(envs, "", "workflow")
	if len(wfs) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(wfs))
	}
}

func TestReadPNGTextChunks(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "meta.png")
	writeTestPNG(t, p, true)
	chunks, err := readPNGTextChunks(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chunks["workflow"], `"TestNode"`) {
		t.Errorf("workflow chunk missing or wrong: %q", chunks["workflow"])
	}
	if chunks["prompt"] == "" {
		t.Error("prompt chunk missing")
	}
}

func TestServerEndToEnd(t *testing.T) {
	envs := newTestEnvs(t)
	srv := httptest.NewServer(NewServer(envs, t.TempDir()).Handler())
	defer srv.Close()

	getJSON := func(path string, into any) {
		t.Helper()
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(into); err != nil {
			t.Fatal(err)
		}
	}
	postJSON := func(path string, body any, into any) int {
		t.Helper()
		data, _ := json.Marshal(body)
		r, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(into)
		return r.StatusCode
	}

	// List
	var list struct {
		Total int     `json:"total"`
		Items []Asset `json:"items"`
	}
	getJSON("/api/assets?env=lounge&type=output", &list)
	if list.Total != 2 {
		t.Fatalf("expected 2 outputs, got %d", list.Total)
	}

	// Thumbnail
	r, err := http.Get(srv.URL + "/api/thumb?env=lounge&type=output&path=gen_001.png")
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("thumb failed: %v status %d", err, r.StatusCode)
	}
	r.Body.Close()

	// Metadata
	var meta struct {
		Meta map[string]string `json:"meta"`
	}
	getJSON("/api/meta?env=lounge&type=output&path=gen_001.png", &meta)
	if meta.Meta["workflow"] == "" {
		t.Fatal("expected embedded workflow metadata")
	}

	// Extract workflow into den
	var extract struct {
		Success bool   `json:"success"`
		Saved   string `json:"saved"`
	}
	postJSON("/api/extract-workflow", map[string]any{
		"items":   []map[string]string{{"env": "lounge", "type": "output", "path": "gen_001.png"}},
		"dst_env": "den",
	}, &extract)
	if !extract.Success || extract.Saved == "" {
		t.Fatalf("extract-workflow failed: %+v", extract)
	}
	if _, err := os.Stat(filepath.Join(envs[1].Path, "user", "default", "workflows", extract.Saved)); err != nil {
		t.Fatalf("extracted workflow not on disk: %v", err)
	}

	// Transfer copy lounge output -> den
	var res struct {
		Success bool     `json:"success"`
		Done    int      `json:"done"`
		Errors  []string `json:"errors"`
	}
	postJSON("/api/transfer", map[string]any{
		"items":   []map[string]string{{"env": "lounge", "type": "output", "path": "gen_001.png"}},
		"dst_env": "den",
	}, &res)
	if !res.Success || res.Done != 1 {
		t.Fatalf("transfer failed: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(envs[1].Path, "output", "gen_001.png")); err != nil {
		t.Fatalf("transferred file not on disk: %v", err)
	}

	// Delete to trash
	postJSON("/api/delete", map[string]any{
		"items": []map[string]string{{"env": "lounge", "type": "output", "path": "sub/gen_002.png"}},
	}, &res)
	if !res.Success {
		t.Fatalf("delete failed: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", trashDirName, "sub", "gen_002.png")); err != nil {
		t.Fatalf("trashed file not in .trash: %v", err)
	}

	// Trash contents must not show up in listings
	getJSON("/api/assets?env=lounge&type=output", &list)
	for _, it := range list.Items {
		if strings.Contains(it.Path, trashDirName) {
			t.Errorf("trash leaked into listing: %s", it.Path)
		}
	}

	// Path traversal must be rejected
	resp, err := http.Get(srv.URL + "/api/file?env=lounge&type=output&path=../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("traversal returned status %d, want 400", resp.StatusCode)
	}

	// Static UI served
	resp2, err := http.Get(srv.URL + "/")
	if err != nil || resp2.StatusCode != 200 {
		t.Fatalf("index not served: %v status %d", err, resp2.StatusCode)
	}
	resp2.Body.Close()
}

func TestFolderOpsAndFilters(t *testing.T) {
	envs := newTestEnvs(t)

	// mkdir + move to folder
	if err := makeFolder(envs, "lounge", "output", "keepers"); err != nil {
		t.Fatal(err)
	}
	if err := moveToFolder(envs, "lounge", "output", "gen_001.png", "keepers"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", "keepers", "gen_001.png")); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}

	// folder-scoped scan
	inFolder := scanFilter(envs, Filter{Env: "lounge", Type: "output", Folder: "keepers"})
	if len(inFolder) != 1 || inFolder[0].Path != "keepers/gen_001.png" {
		t.Fatalf("folder scan wrong: %+v", inFolder)
	}

	// subfolders listing
	subs := subfolders(envs, Filter{Env: "lounge", Type: "output"})
	if len(subs) != 2 { // keepers + sub
		t.Fatalf("expected 2 subfolders, got %v", subs)
	}

	// rename
	if err := renameAsset(envs, "lounge", "output", "keepers/gen_001.png", "hero.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", "keepers", "hero.png")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if err := renameAsset(envs, "lounge", "output", "keepers/hero.png", "../escape.png"); err == nil {
		t.Fatal("rename with path separator should be rejected")
	}

	// date filter: everything is recent, so a future From excludes all
	future := scanFilter(envs, Filter{Env: "lounge", From: time.Now().Add(time.Hour)})
	if len(future) != 0 {
		t.Fatalf("future date filter should match nothing, got %d", len(future))
	}
}

func TestDeleteFolder(t *testing.T) {
	envs := newTestEnvs(t)

	// keep contents: sub/gen_002.png moves up to the output root
	if err := deleteFolder(envs, "lounge", "output", "sub", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", "gen_002.png")); err != nil {
		t.Fatalf("content should have moved up: %v", err)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", "sub")); !os.IsNotExist(err) {
		t.Fatal("folder should be gone")
	}

	// delete everything: folder + contents go to .trash
	if err := makeFolder(envs, "lounge", "output", "junk"); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(envs[0].Path, "output", "junk", "x.png"), false)
	if err := deleteFolder(envs, "lounge", "output", "junk", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", trashDirName, "junk", "x.png")); err != nil {
		t.Fatalf("folder contents should be in .trash: %v", err)
	}

	// guard rails
	if err := deleteFolder(envs, "lounge", "output", "", false); err == nil {
		t.Fatal("deleting the root must be rejected")
	}
	if err := deleteFolder(envs, "lounge", "output", "../input", false); err == nil {
		t.Fatal("traversal must be rejected")
	}
}

func TestRemoveEmptyFolders(t *testing.T) {
	envs := newTestEnvs(t)
	// a/b/c is empty all the way down; "sub" contains a file and must survive.
	if err := os.MkdirAll(filepath.Join(envs[0].Path, "output", "a", "b", "c"), 0755); err != nil {
		t.Fatal(err)
	}
	// .trash must never be touched
	if err := os.MkdirAll(filepath.Join(envs[0].Path, "output", trashDirName, "old"), 0755); err != nil {
		t.Fatal(err)
	}

	found := findEmptyFolders(envs, "lounge", "output")
	if len(found) != 3 { // a, a/b, a/b/c
		t.Fatalf("expected 3 empty folders, got %v", found)
	}
	if found[0].Folder != "a/b/c" {
		t.Fatalf("deepest folder should come first, got %v", found)
	}

	removed := removeEmptyFolders(envs, "lounge", "output")
	if len(removed) != 3 {
		t.Fatalf("expected 3 removed, got %v", removed)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", "a")); !os.IsNotExist(err) {
		t.Fatal("empty tree should be gone")
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", "sub")); err != nil {
		t.Fatal("folder with content must survive")
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", trashDirName, "old")); err != nil {
		t.Fatal(".trash must be untouched")
	}
}

func TestFindDuplicates(t *testing.T) {
	envs := newTestEnvs(t)
	// gen_dup.png is byte-identical to sub/gen_002.png (same generator, no meta)
	src := filepath.Join(envs[0].Path, "output", "sub", "gen_002.png")
	dup := filepath.Join(envs[0].Path, "output", "gen_dup.png")
	data, _ := os.ReadFile(src)
	if err := os.WriteFile(dup, data, 0644); err != nil {
		t.Fatal(err)
	}

	hc := loadHashCache(filepath.Join(t.TempDir(), "hash-cache.json"))
	groups := findDuplicates(envs, Filter{Env: "lounge", Type: "output"}, hc)
	if len(groups) != 1 {
		t.Fatalf("expected 1 dupe group, got %d", len(groups))
	}
	if len(groups[0].Items) != 2 {
		t.Fatalf("expected 2 items in group, got %d", len(groups[0].Items))
	}
	// Second run hits the cache (no error, same result).
	groups2 := findDuplicates(envs, Filter{Env: "lounge", Type: "output"}, hc)
	if len(groups2) != 1 {
		t.Fatalf("cached run differs: %d groups", len(groups2))
	}
}

func TestHTMLViewsAndFilterBulk(t *testing.T) {
	envs := newTestEnvs(t)
	srv := httptest.NewServer(NewServer(envs, t.TempDir()).Handler())
	defer srv.Close()

	get := func(path string) (int, string) {
		t.Helper()
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var sb strings.Builder
		if _, err := io.Copy(&sb, r.Body); err != nil {
			t.Fatal(err)
		}
		return r.StatusCode, sb.String()
	}

	// Page shell
	code, body := get("/")
	if code != 200 || !strings.Contains(body, "htmx.min.js") || !strings.Contains(body, `id="grid"`) {
		t.Fatalf("layout wrong: code=%d", code)
	}

	// Grid fragment with day grouping and cards
	code, body = get("/grid?env=lounge&type=output")
	if code != 200 || !strings.Contains(body, `class="card"`) || !strings.Contains(body, "Today") {
		t.Fatalf("grid fragment wrong: code=%d body=%s", code, body[:min(300, len(body))])
	}
	if !strings.Contains(body, "folderbar") || !strings.Contains(body, "📁 sub") {
		t.Fatal("grid should include the folder bar with the sub folder chip")
	}

	// Detail fragment
	code, body = get("/detail?env=lounge&type=output&path=gen_001.png")
	if code != 200 || !strings.Contains(body, "Save workflow to env") {
		t.Fatalf("detail fragment should offer workflow extraction: code=%d", code)
	}

	// Dupes fragment (no dupes in default fixture)
	code, body = get("/dupes?env=lounge&type=output")
	if code != 200 || !strings.Contains(body, "No duplicates") {
		t.Fatalf("dupes fragment wrong: code=%d", code)
	}

	// Filter-based bulk: trash everything in lounge/output via filter
	data, _ := json.Marshal(map[string]any{"filter": map[string]string{"env": "lounge", "type": "output"}})
	r, err := http.Post(srv.URL+"/api/delete", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Success bool `json:"success"`
		Done    int  `json:"done"`
	}
	_ = json.NewDecoder(r.Body).Decode(&res)
	r.Body.Close()
	if !res.Success || res.Done != 2 {
		t.Fatalf("filter bulk delete failed: %+v", res)
	}
	if left := scan(envs, "lounge", "output"); len(left) != 0 {
		t.Fatalf("expected empty output after filter delete, got %d", len(left))
	}
}

func TestEmptyTrash(t *testing.T) {
	envs := newTestEnvs(t)
	if err := moveToTrash(envs, "lounge", "output", "gen_001.png"); err != nil {
		t.Fatal(err)
	}
	if err := moveToTrash(envs, "lounge", "input", "ref.png"); err != nil {
		t.Fatal(err)
	}

	// Scoped empty: only the output trash goes away.
	files, bytes := EmptyTrash(envs, "lounge", "output")
	if files != 1 || bytes <= 0 {
		t.Fatalf("expected 1 trashed output file freed, got files=%d bytes=%d", files, bytes)
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "output", trashDirName)); !os.IsNotExist(err) {
		t.Error("output trash dir still exists")
	}
	if _, err := os.Stat(filepath.Join(envs[0].Path, "input", trashDirName, "ref.png")); err != nil {
		t.Error("input trash should have been untouched")
	}

	// Unscoped empty clears the rest.
	files, _ = EmptyTrash(envs, "", "")
	if files != 1 {
		t.Fatalf("expected 1 remaining trashed file, got %d", files)
	}
}

func TestVideoThumbnail(t *testing.T) {
	if ffmpegPath() == "" {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	// Generate a 2-second test clip.
	gen := exec.Command(ffmpegPath(), "-f", "lavfi", "-i", "testsrc=duration=2:size=64x64:rate=10", "-y", video)
	if err := gen.Run(); err != nil {
		t.Skipf("could not generate test video: %v", err)
	}
	thumb, err := videoThumbnail(t.TempDir(), video)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(thumb)
	if err != nil || info.Size() == 0 {
		t.Fatalf("video thumbnail missing or empty: %v", err)
	}
}

func TestThumbnailCache(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "img.png")
	writeTestPNG(t, p, false)
	cache := t.TempDir()
	first, err := thumbnail(cache, p)
	if err != nil {
		t.Fatal(err)
	}
	second, err := thumbnail(cache, p)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("cache miss on identical file: %s vs %s", first, second)
	}
	if fmt.Sprintf("%v", filepath.Ext(first)) != ".jpg" {
		t.Errorf("thumbnail is not a jpg: %s", first)
	}
}
