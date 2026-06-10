package assets

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

// hashEntry caches the content hash of a file at a given mtime/size, so
// dedupe scans only re-read files that changed.
type hashEntry struct {
	MTime int64  `json:"mtime"`
	Size  int64  `json:"size"`
	SHA1  string `json:"sha1"`
}

type hashCache struct {
	path  string
	mu    sync.Mutex
	m     map[string]hashEntry
	dirty bool
}

func loadHashCache(path string) *hashCache {
	hc := &hashCache{path: path, m: map[string]hashEntry{}}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &hc.m)
	}
	return hc
}

func (hc *hashCache) save() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if !hc.dirty {
		return
	}
	data, err := json.Marshal(hc.m)
	if err != nil {
		return
	}
	_ = os.WriteFile(hc.path, data, 0644)
	hc.dirty = false
}

// fileSHA1 returns the cached hash, re-hashing when the file changed.
func (hc *hashCache) fileSHA1(absPath string) (string, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	hc.mu.Lock()
	if e, ok := hc.m[absPath]; ok && e.MTime == info.ModTime().UnixNano() && e.Size == info.Size() {
		hc.mu.Unlock()
		return e.SHA1, nil
	}
	hc.mu.Unlock()

	f, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := fmt.Sprintf("%x", h.Sum(nil))

	hc.mu.Lock()
	hc.m[absPath] = hashEntry{MTime: info.ModTime().UnixNano(), Size: info.Size(), SHA1: sum}
	hc.dirty = true
	hc.mu.Unlock()
	return sum, nil
}

// DupeGroup is a set of byte-identical assets.
type DupeGroup struct {
	Hash  string
	Size  int64
	Items []Asset
}

// findDuplicates hashes the assets matching the filter and returns groups of
// identical files (newest first inside a group). Files with unique sizes are
// skipped without hashing, so the common case stays cheap.
func findDuplicates(envs []Env, f Filter, hc *hashCache) []DupeGroup {
	items := scanFilter(envs, f)

	bySize := map[int64][]Asset{}
	for _, a := range items {
		bySize[a.Size] = append(bySize[a.Size], a)
	}

	const workers = 8
	type hashed struct {
		asset Asset
		sum   string
	}
	jobs := make(chan Asset)
	results := make(chan hashed)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for a := range jobs {
				abs, err := resolveAssetPath(envs, a.Env, a.Type, a.Path)
				if err != nil {
					continue
				}
				sum, err := hc.fileSHA1(abs)
				if err != nil {
					continue
				}
				results <- hashed{a, sum}
			}
		}()
	}
	go func() {
		for _, group := range bySize {
			if len(group) < 2 {
				continue // unique size cannot have a duplicate
			}
			for _, a := range group {
				jobs <- a
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	byHash := map[string][]Asset{}
	for r := range results {
		byHash[r.sum] = append(byHash[r.sum], r.asset)
	}
	hc.save()

	var out []DupeGroup
	for sum, group := range byHash {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].ModTime.After(group[j].ModTime) })
		out = append(out, DupeGroup{Hash: sum, Size: group[0].Size, Items: group})
	}
	// Largest reclaimable space first.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Size*int64(len(out[i].Items)-1) > out[j].Size*int64(len(out[j].Items)-1)
	})
	return out
}
