package assets

import (
	"crypto/sha1"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"golang.org/x/image/draw"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

const thumbMaxDim = 448

// ffmpegPath resolves ffmpeg once; empty when not installed.
var ffmpegPath = sync.OnceValue(func() string {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return p
})

// cachePathFor builds the cache file path for an asset; the key includes the
// file's mtime/size (so edits invalidate stale thumbs) and the thumbnail
// dimension (so bumping thumbMaxDim regenerates them).
func cachePathFor(cacheDir, absPath string) (string, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	key := sha1.Sum(fmt.Appendf(nil, "%s|%d|%d|%d", absPath, info.ModTime().UnixNano(), info.Size(), thumbMaxDim))
	return filepath.Join(cacheDir, fmt.Sprintf("%x.jpg", key)), nil
}

// videoThumbnail extracts a poster frame with ffmpeg and caches it as JPEG.
func videoThumbnail(cacheDir, absPath string) (string, error) {
	ff := ffmpegPath()
	if ff == "" {
		return "", fmt.Errorf("ffmpeg not installed; no video thumbnails")
	}
	cached, err := cachePathFor(cacheDir, absPath)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	tmp := cached + ".tmp.jpg" // .jpg suffix so ffmpeg infers the format
	scale := fmt.Sprintf("scale='min(%d,iw)':-2", thumbMaxDim)
	// Fast-seek 1s in for a representative frame; very short clips fall back to frame 0.
	for _, args := range [][]string{
		{"-ss", "1", "-i", absPath, "-frames:v", "1", "-vf", scale, "-q:v", "4", "-y", tmp},
		{"-i", absPath, "-frames:v", "1", "-vf", scale, "-q:v", "4", "-y", tmp},
	} {
		cmd := exec.Command(ff, args...)
		if err := cmd.Run(); err == nil {
			if info, err := os.Stat(tmp); err == nil && info.Size() > 0 {
				if err := os.Rename(tmp, cached); err != nil {
					return "", err
				}
				return cached, nil
			}
		}
	}
	os.Remove(tmp)
	return "", fmt.Errorf("ffmpeg could not extract a frame from %s", filepath.Base(absPath))
}

// thumbnail returns the path of a cached JPEG thumbnail for the image at
// absPath, generating it on first access.
func thumbnail(cacheDir, absPath string) (string, error) {
	cached, err := cachePathFor(cacheDir, absPath)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", filepath.Base(absPath), err)
	}

	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > thumbMaxDim || h > thumbMaxDim {
		if w >= h {
			h = h * thumbMaxDim / w
			w = thumbMaxDim
		} else {
			w = w * thumbMaxDim / h
			h = thumbMaxDim
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
		src = dst
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	tmp := cached + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if err := jpeg.Encode(out, src, &jpeg.Options{Quality: 82}); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, cached); err != nil {
		return "", err
	}
	return cached, nil
}
