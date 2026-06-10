package assets

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed web
var webFS embed.FS

// Server serves the asset manager web UI and JSON API for a set of
// ComfyUI environments. It binds to localhost only.
type Server struct {
	envs     []Env
	cacheDir string
}

// NewServer creates an asset manager server for the given environments.
func NewServer(envs []Env, cacheDir string) *Server {
	return &Server{envs: envs, cacheDir: cacheDir}
}

// Handler returns the http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/envs", s.handleEnvs)
	mux.HandleFunc("GET /api/assets", s.handleList)
	mux.HandleFunc("GET /api/thumb", s.handleThumb)
	mux.HandleFunc("GET /api/file", s.handleFile)
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("POST /api/delete", s.handleDelete)
	mux.HandleFunc("POST /api/transfer", s.handleTransfer)
	mux.HandleFunc("POST /api/extract-workflow", s.handleExtractWorkflow)
	mux.HandleFunc("POST /api/empty-trash", s.handleEmptyTrash)

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

type batchRequest struct {
	Items  []itemRef `json:"items"`
	DstEnv string    `json:"dst_env"`
	Move   bool      `json:"move"`
	Name   string    `json:"name"`
}

func decodeBatch(r *http.Request) (batchRequest, error) {
	var req batchRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err == nil && len(req.Items) == 0 {
		err = fmt.Errorf("no items provided")
	}
	return req, err
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
	req, err := decodeBatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	batchResult(w, req.Items, func(it itemRef) error {
		return moveToTrash(s.envs, it.Env, it.Type, it.Path)
	})
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBatch(r)
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
	req, err := decodeBatch(r)
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
