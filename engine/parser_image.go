package engine

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImageParser implements FileParser for image files
type ImageParser struct{}

// Detect checks if file is an image
func (p *ImageParser) Detect(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, e := range imageExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// Parse is not used for images (binary content); returns nil
func (p *ImageParser) Parse(content string) []Block {
	return nil
}

// ParseFile reads an image file and returns a single Block with ImageData
// static=true inlines as base64 data URI; static=false stores file path for server mode
func (p *ImageParser) ParseFile(filePath string, static bool) ([]Block, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not resolve path: %w", err)
	}

	title := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	mime := ImageMIME(ext)

	var src string
	inline := false

	if static {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("could not read file: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		src = fmt.Sprintf("data:%s;base64,%s", mime, b64)
		inline = true
	} else {
		src = absPath
	}

	block := Block{
		Name:        title,
		Content:     src,
		Pages:       []string{src},
		TotalPages:  1,
		ContentType: BlockContentImage,
		Data: &ImageData{
			Src:    src,
			MIME:   mime,
			Alt:    title,
			Inline: inline,
		},
	}

	return []Block{block}, nil
}
