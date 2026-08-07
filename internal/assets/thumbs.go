package assets

import (
	"context"
	"crypto/sha1"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/image/draw"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

const (
	thumbMaxDim = 448
	// srcMaxDim rejects decompression-bomb images before allocating pixels.
	srcMaxDim = 10000
	// ffmpegTimeout bounds a single poster-frame extraction.
	ffmpegTimeout = 15 * time.Second
)

// thumbGen caps how many thumbnails are generated at once, so a browser
// scrolling a folder of videos cannot fork an ffmpeg per visible tile.
var thumbGen = make(chan struct{}, 4)

func acquireThumbSlot() func() {
	thumbGen <- struct{}{}
	return func() { <-thumbGen }
}

// writeCached writes via a unique temp file in the cache dir and renames into
// place, so concurrent requests for the same asset cannot corrupt each other.
func writeCached(cacheDir, cached string, write func(tmp string) error) error {
	f, err := os.CreateTemp(cacheDir, "thumb-*.jpg")
	if err != nil {
		return err
	}
	tmp := f.Name()
	f.Close()
	defer os.Remove(tmp)
	if err := write(tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, cached); err != nil {
		return err
	}
	return nil
}

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
	// A video ffmpeg already failed on stays failed until it changes (the cache
	// key includes mtime/size), so don't respawn ffmpeg on every request.
	failMarker := cached + ".fail"
	if _, err := os.Stat(failMarker); err == nil {
		return "", fmt.Errorf("no thumbnail available for %s", filepath.Base(absPath))
	}

	release := acquireThumbSlot()
	defer release()
	// Another request may have generated it while we waited for a slot.
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	scale := fmt.Sprintf("scale='min(%d,iw)':-2", thumbMaxDim)
	err = writeCached(cacheDir, cached, func(tmp string) error {
		// Fast-seek 1s in for a representative frame; very short clips fall back to frame 0.
		for _, args := range [][]string{
			{"-ss", "1", "-i", absPath, "-frames:v", "1", "-vf", scale, "-q:v", "4", "-y", tmp},
			{"-i", absPath, "-frames:v", "1", "-vf", scale, "-q:v", "4", "-y", tmp},
		} {
			ctx, cancel := context.WithTimeout(context.Background(), ffmpegTimeout)
			runErr := exec.CommandContext(ctx, ff, args...).Run()
			cancel()
			if runErr != nil {
				continue
			}
			if info, err := os.Stat(tmp); err == nil && info.Size() > 0 {
				return nil
			}
		}
		return fmt.Errorf("ffmpeg could not extract a frame from %s", filepath.Base(absPath))
	})
	if err != nil {
		_ = os.WriteFile(failMarker, nil, 0644)
		return "", err
	}
	return cached, nil
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

	release := acquireThumbSlot()
	defer release()
	if _, err := os.Stat(cached); err == nil {
		return cached, nil // generated while we waited for a slot
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	// Check the header before decoding: a small file can declare a huge canvas.
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", filepath.Base(absPath), err)
	}
	if cfg.Width > srcMaxDim || cfg.Height > srcMaxDim {
		return "", fmt.Errorf("%s is too large to thumbnail (%dx%d)", filepath.Base(absPath), cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
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
	err = writeCached(cacheDir, cached, func(tmp string) error {
		out, err := os.Create(tmp)
		if err != nil {
			return err
		}
		if err := jpeg.Encode(out, src, &jpeg.Options{Quality: 82}); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
	if err != nil {
		return "", err
	}
	return cached, nil
}
