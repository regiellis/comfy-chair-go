package assets

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed web
var webFS embed.FS

// Server serves the asset manager web UI (server-rendered htmx fragments)
// and JSON action API for a set of ComfyUI environments.
type Server struct {
	envs     []Env
	cacheDir string
	hashes   *hashCache
}

// NewServer creates an asset manager server for the given environments.
func NewServer(envs []Env, cacheDir string) *Server {
	_ = os.MkdirAll(cacheDir, 0755)
	return &Server{
		envs:     envs,
		cacheDir: cacheDir,
		hashes:   loadHashCache(filepath.Join(cacheDir, "hash-cache.json")),
	}
}

// Handler returns the http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// HTML views (htmx fragments)
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /grid", s.handleGrid)
	mux.HandleFunc("GET /detail", s.handleDetail)
	mux.HandleFunc("GET /dupes", s.handleDupes)
	// JSON API
	mux.HandleFunc("GET /api/envs", s.handleEnvs)
	mux.HandleFunc("GET /api/assets", s.handleList)
	mux.HandleFunc("GET /api/thumb", s.handleThumb)
	mux.HandleFunc("GET /api/file", s.handleFile)
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("POST /api/delete", s.handleDelete)
	mux.HandleFunc("POST /api/transfer", s.handleTransfer)
	mux.HandleFunc("POST /api/extract-workflow", s.handleExtractWorkflow)
	mux.HandleFunc("POST /api/empty-trash", s.handleEmptyTrash)
	mux.HandleFunc("POST /api/rename", s.handleRename)
	mux.HandleFunc("POST /api/mkdir", s.handleMkdir)
	mux.HandleFunc("POST /api/move-folder", s.handleMoveFolder)
	mux.HandleFunc("POST /api/delete-folder", s.handleDeleteFolder)
	mux.HandleFunc("POST /api/clean-folders", s.handleCleanFolders)

	web, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /", http.FileServerFS(web))
	return mux
}

// Listen serves until the process exits. The API can read and modify files
// in the configured environments and has no auth layer — callers should warn
// loudly when host is anything other than a loopback address.
func (s *Server) Listen(host string, port int) error {
	return http.ListenAndServe(net.JoinHostPort(host, strconv.Itoa(port)), s.Handler())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"success": false, "error": err.Error()})
}

func (s *Server) handleEnvs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "envs": s.envs})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items := scan(s.envs, q.Get("env"), q.Get("type"))

	if search := strings.ToLower(strings.TrimSpace(q.Get("q"))); search != "" {
		filtered := items[:0]
		for _, a := range items {
			if strings.Contains(strings.ToLower(a.Path), search) {
				filtered = append(filtered, a)
			}
		}
		items = filtered
	}
	sortAssets(items, q.Get("sort"))

	total := len(items)
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := items[offset:end]
	if page == nil {
		page = []Asset{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total": total, "items": page})
}

// assetFromQuery resolves the env/type/path triple every file-level endpoint takes.
func (s *Server) assetFromQuery(r *http.Request) (abs string, err error) {
	q := r.URL.Query()
	return resolveAssetPath(s.envs, q.Get("env"), q.Get("type"), q.Get("path"))
}

func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	abs, err := s.assetFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var thumb string
	if kindForExt[strings.ToLower(filepath.Ext(abs))] == "video" {
		thumb, err = videoThumbnail(s.cacheDir, abs)
	} else {
		thumb, err = thumbnail(s.cacheDir, abs)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeFile(w, r, thumb)
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	req, err := s.decodeBatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	it := req.Items[0]
	if err := renameAsset(s.envs, it.Env, it.Type, it.Path, req.Name); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Env    string `json:"env"`
		Type   string `json:"type"`
		Folder string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Folder == "" || req.Type == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("env, type, and folder are required"))
		return
	}
	if req.Env == "" {
		// "All environments" view: create the folder in every env.
		var errs []string
		for _, e := range s.envs {
			if err := makeFolder(s.envs, e.Name, req.Type, req.Folder); err != nil {
				errs = append(errs, err.Error())
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": len(errs) == 0, "errors": errs})
		return
	}
	if err := makeFolder(s.envs, req.Env, req.Type, req.Folder); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleMoveFolder(w http.ResponseWriter, r *http.Request) {
	req, err := s.decodeBatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	batchResult(w, req.Items, func(it itemRef) error {
		return moveToFolder(s.envs, it.Env, it.Type, it.Path, req.Folder)
	})
}

func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Env          string `json:"env"`
		Type         string `json:"type"`
		Folder       string `json:"folder"`
		KeepContents bool   `json:"keep_contents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Folder == "" || req.Type == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("type and folder are required"))
		return
	}
	targets := []string{req.Env}
	if req.Env == "" {
		// "All environments" view: delete the folder wherever it exists.
		targets = targets[:0]
		for _, e := range s.envs {
			if abs, err := resolveAssetPath(s.envs, e.Name, req.Type, req.Folder); err == nil {
				if info, err := os.Stat(abs); err == nil && info.IsDir() {
					targets = append(targets, e.Name)
				}
			}
		}
		if len(targets) == 0 {
			writeErr(w, http.StatusNotFound, fmt.Errorf("folder %s not found in any environment", req.Folder))
			return
		}
	}
	var errs []string
	for _, env := range targets {
		if err := deleteFolder(s.envs, env, req.Type, req.Folder, req.KeepContents); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", env, err))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": len(errs) == 0, "errors": errs})
}

// handleCleanFolders removes empty folders in scope. With dry=true it only
// reports what would be removed, so the UI can confirm with a real count.
func (s *Server) handleCleanFolders(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Env  string `json:"env"`
		Type string `json:"type"`
		Dry  bool   `json:"dry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var folders []folderRef
	if req.Dry {
		folders = findEmptyFolders(s.envs, req.Env, req.Type)
	} else {
		folders = removeEmptyFolders(s.envs, req.Env, req.Type)
	}
	if folders == nil {
		folders = []folderRef{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "folders": folders})
}

func (s *Server) handleEmptyTrash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Env  string `json:"env"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	files, bytes := EmptyTrash(s.envs, req.Env, req.Type)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "files": files, "bytes": bytes})
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	abs, err := s.assetFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// http.ServeFile provides range requests, which video playback needs.
	http.ServeFile(w, r, abs)
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	abs, err := s.assetFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	chunks, err := readPNGTextChunks(abs)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "meta": map[string]string{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "meta": chunks})
}

type itemRef struct {
	Env  string `json:"env"`
	Type string `json:"type"`
	Path string `json:"path"`
}

// filterRef mirrors the filter form fields for "act on everything matching".
type filterRef struct {
	Env    string `json:"env"`
	Type   string `json:"type"`
	Query  string `json:"q"`
	Folder string `json:"folder"`
	From   string `json:"from"`
	To     string `json:"to"`
}

func (fr *filterRef) toFilter() Filter {
	v := url.Values{}
	v.Set("env", fr.Env)
	v.Set("type", fr.Type)
	v.Set("q", fr.Query)
	v.Set("folder", fr.Folder)
	v.Set("from", fr.From)
	v.Set("to", fr.To)
	return parseFilter(v)
}

type batchRequest struct {
	Items  []itemRef  `json:"items"`
	Filter *filterRef `json:"filter"`
	DstEnv string     `json:"dst_env"`
	Move   bool       `json:"move"`
	Name   string     `json:"name"`
	Folder string     `json:"folder"`
}

// decodeBatch parses a batch action; when a filter is given instead of items,
// it resolves to every matching asset so bulk actions cover the full result
// set, not just the cards loaded in the browser.
func (s *Server) decodeBatch(r *http.Request) (batchRequest, error) {
	var req batchRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return req, err
	}
	if len(req.Items) == 0 && req.Filter != nil {
		for _, a := range scanFilter(s.envs, req.Filter.toFilter()) {
			req.Items = append(req.Items, itemRef{Env: a.Env, Type: a.Type, Path: a.Path})
		}
	}
	if len(req.Items) == 0 {
		return req, fmt.Errorf("no items matched")
	}
	return req, nil
}

// batchResult applies fn to every item and reports per-item errors.
func batchResult(w http.ResponseWriter, items []itemRef, fn func(itemRef) error) {
	var errs []string
	done := 0
	for _, it := range items {
		if err := fn(it); err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s/%s: %v", it.Env, it.Type, it.Path, err))
		} else {
			done++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": len(errs) == 0, "done": done, "errors": errs})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	req, err := s.decodeBatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	batchResult(w, req.Items, func(it itemRef) error {
		return moveToTrash(s.envs, it.Env, it.Type, it.Path)
	})
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	req, err := s.decodeBatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.DstEnv == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("dst_env is required"))
		return
	}
	batchResult(w, req.Items, func(it itemRef) error {
		return transfer(s.envs, it.Env, it.Type, it.Path, req.DstEnv, req.Move)
	})
}

func (s *Server) handleExtractWorkflow(w http.ResponseWriter, r *http.Request) {
	req, err := s.decodeBatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.DstEnv == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("dst_env is required"))
		return
	}
	it := req.Items[0]
	abs, err := resolveAssetPath(s.envs, it.Env, it.Type, it.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	chunks, err := readPNGTextChunks(abs)
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("could not read PNG metadata: %w", err))
		return
	}
	workflow, ok := chunks["workflow"]
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("no embedded workflow found in %s", it.Path))
		return
	}
	name := req.Name
	if name == "" {
		name = strings.TrimSuffix(it.Path, ".png")
		name = strings.ReplaceAll(name, "/", "_")
	}
	saved, err := saveWorkflowJSON(s.envs, req.DstEnv, name, []byte(workflow))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "saved": saved})
}
