package assets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// readPNGTextChunks returns the tEXt key/value pairs of a PNG file. ComfyUI
// stores the API prompt under "prompt" and the editor graph under "workflow".
func readPNGTextChunks(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sig := make([]byte, len(pngSignature))
	if _, err := io.ReadFull(f, sig); err != nil || !bytes.Equal(sig, pngSignature) {
		return nil, fmt.Errorf("not a PNG file")
	}

	out := map[string]string{}
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(f, header); err != nil {
			break // EOF or truncated file: return what we have
		}
		length := binary.BigEndian.Uint32(header[:4])
		chunkType := string(header[4:8])
		if chunkType == "IEND" {
			break
		}
		// Metadata chunks precede image data; stop early instead of reading pixels.
		if chunkType == "IDAT" {
			break
		}
		if chunkType == "tEXt" && length < 64*1024*1024 {
			data := make([]byte, length)
			if _, err := io.ReadFull(f, data); err != nil {
				break
			}
			if i := bytes.IndexByte(data, 0); i > 0 {
				out[string(data[:i])] = string(data[i+1:])
			}
			if _, err := f.Seek(4, io.SeekCurrent); err != nil { // CRC
				break
			}
			continue
		}
		if _, err := f.Seek(int64(length)+4, io.SeekCurrent); err != nil { // data + CRC
			break
		}
	}
	return out, nil
}
