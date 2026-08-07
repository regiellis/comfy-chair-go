package assets

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

var tmpl = template.Must(template.ParseFS(webFS, "web/templates/*.tmpl"))

const pageSize = 120

// cardVM is the per-asset view model used by grid, dupes, and detail templates.
type cardVM struct {
	Asset
	ThumbURL     string
	FileURL      string
	FallbackIcon string
	Thumbable    bool
	SizeHuman    string
	DateHuman    string
}

func assetQuery(a Asset) string {
	v := url.Values{}
	v.Set("env", a.Env)
	v.Set("type", a.Type)
	v.Set("path", a.Path)
	return v.Encode()
}

func sizeHuman(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func newCardVM(a Asset) cardVM {
	icon := "🧩"
	switch a.Kind {
	case "image":
		icon = "🖼"
	case "video":
		icon = "🎬"
	case "audio":
		icon = "🎵"
	}
	return cardVM{
		Asset:        a,
		ThumbURL:     "/api/thumb?" + assetQuery(a),
		FileURL:      "/api/file?" + assetQuery(a),
		FallbackIcon: icon,
		Thumbable:    a.Kind == "image" || a.Kind == "video",
		SizeHuman:    sizeHuman(a.Size),
		DateHuman:    a.ModTime.Local().Format("Jan 2, 15:04"),
	}
}

// parseFilter builds a Filter from query/form values.
func parseFilter(v url.Values) Filter {
	f := Filter{
		Env:    v.Get("env"),
		Type:   v.Get("type"),
		Query:  v.Get("q"),
		Folder: v.Get("folder"),
		Sort:   v.Get("sort"),
	}
	if from, err := time.ParseInLocation("2006-01-02", v.Get("from"), time.Local); err == nil {
		f.From = from
	}
	if to, err := time.ParseInLocation("2006-01-02", v.Get("to"), time.Local); err == nil {
		f.To = to.AddDate(0, 0, 1) // inclusive end of day
	}
	return f
}

type crumbVM struct{ Name, Folder string }
type folderVM struct{ Name, Folder string }

type groupVM struct {
	Label string
	Items []cardVM
}

type gridVM struct {
	Groups        []groupVM
	Total         int
	Query         string
	HasMore       bool
	NextOffset    int
	ShowHeader    bool
	ShowFolderBar bool
	TypeLabel     string
	Crumbs        []crumbVM
	Subfolders    []folderVM
}

func (s *Server) handleHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", map[string]any{"Envs": s.envs}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGrid(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r.URL.Query())
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	items := scanFilter(s.envs, f)
	total := len(items)
	end := min(offset+pageSize, total)
	page := items[min(offset, total):end]

	// Day-group headers only make sense in date order. A page that starts
	// mid-group omits the label so infinite scroll doesn't repeat headers.
	grouped := f.Sort == "" || f.Sort == "date"
	now := time.Now()
	var groups []groupVM
	prevLabel := ""
	// An offset past the end happens on a stale infinite-scroll page (files
	// deleted between requests) as well as on a hand-crafted URL.
	if grouped && offset > 0 && offset-1 < total {
		prevLabel = GroupLabel(items[offset-1].ModTime, now)
	}
	for _, a := range page {
		label := ""
		if grouped {
			label = GroupLabel(a.ModTime, now)
		}
		if len(groups) == 0 || (grouped && label != prevLabel) {
			head := label
			if label == prevLabel {
				head = "" // continuation across the page boundary
			}
			groups = append(groups, groupVM{Label: head})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, newCardVM(a))
		prevLabel = label
	}

	vm := gridVM{
		Groups:     groups,
		Total:      total,
		Query:      f.Query,
		HasMore:    end < total,
		NextOffset: end,
		ShowHeader: offset == 0,
	}
	if offset == 0 {
		vm.ShowFolderBar = f.Type != "" // folders are per-type
		vm.TypeLabel = map[string]string{"output": "Outputs", "input": "Inputs", "workflow": "Workflows"}[f.Type]
		folder := strings.Trim(path.Clean("/"+f.Folder), "/")
		if folder != "" {
			acc := ""
			for _, part := range strings.Split(folder, "/") {
				acc = path.Join(acc, part)
				vm.Crumbs = append(vm.Crumbs, crumbVM{Name: part, Folder: acc})
			}
		}
		for _, name := range subfolders(s.envs, f) {
			vm.Subfolders = append(vm.Subfolders, folderVM{Name: name, Folder: path.Join(folder, name)})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "grid", vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type detailVM struct {
	cardVM
	Envs         []Env
	HasWorkflow  bool
	Prompt       string
	WorkflowJSON string
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	abs, err := resolveAssetPath(s.envs, q.Get("env"), q.Get("type"), q.Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items := scanFilter(s.envs, Filter{Env: q.Get("env"), Type: q.Get("type"), Query: ""})
	var asset *Asset
	for i := range items {
		if items[i].Path == q.Get("path") {
			asset = &items[i]
			break
		}
	}
	if asset == nil {
		http.Error(w, "asset not found", http.StatusNotFound)
		return
	}

	vm := detailVM{cardVM: newCardVM(*asset), Envs: s.envs}
	vm.DateHuman = asset.ModTime.Local().Format("Jan 2 2006, 15:04:05")
	if asset.Kind == "image" && strings.HasSuffix(strings.ToLower(asset.Name), ".png") {
		if chunks, err := readPNGTextChunks(abs); err == nil {
			_, vm.HasWorkflow = chunks["workflow"]
			if p, ok := chunks["prompt"]; ok {
				vm.Prompt = prettyJSON(p, 64*1024)
			}
		}
	}
	if asset.Kind == "workflow" {
		if data, err := readFileCapped(abs, 1<<20); err == nil {
			vm.WorkflowJSON = prettyJSON(string(data), 1<<20)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "detail", vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type dupeGroupVM struct {
	Items     []cardVM
	SizeHuman string
}

func (s *Server) handleDupes(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r.URL.Query())
	groups := findDuplicates(s.envs, f, s.hashes)

	var vms []dupeGroupVM
	var reclaimable int64
	for _, g := range groups {
		vm := dupeGroupVM{SizeHuman: sizeHuman(g.Size)}
		for _, a := range g.Items {
			vm.Items = append(vm.Items, newCardVM(a))
		}
		reclaimable += g.Size * int64(len(g.Items)-1)
		vms = append(vms, vm)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.ExecuteTemplate(w, "dupes", map[string]any{
		"Groups":      vms,
		"Reclaimable": sizeHuman(reclaimable),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func prettyJSON(raw string, capBytes int) string {
	if len(raw) > capBytes {
		return raw[:capBytes] + "\n… (truncated)"
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return raw
	}
	return string(pretty)
}
