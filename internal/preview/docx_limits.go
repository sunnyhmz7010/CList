package preview

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

var ErrArchiveLimit = errors.New("DOCX archive exceeds preview limits")

const (
	maxDocxBytes        = 32 << 20
	maxDocxFiles        = 1000
	maxExpanded         = 64 << 20
	maxRatio     uint64 = 1000
)

func ValidateDocx(reader io.Reader) error {
	raw, err := io.ReadAll(io.LimitReader(reader, maxDocxBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxDocxBytes {
		return ErrArchiveLimit
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return ErrArchiveLimit
	}
	if len(archive.File) > maxDocxFiles {
		return ErrArchiveLimit
	}
	var expanded uint64
	for _, file := range archive.File {
		expanded += file.UncompressedSize64
		if expanded > maxExpanded || (file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > maxRatio) {
			return ErrArchiveLimit
		}
	}
	return nil
}

func ExtractDocxText(reader io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxDocxBytes+1))
	if err != nil {
		return "", err
	}
	if err := ValidateDocx(bytes.NewReader(raw)); err != nil {
		return "", err
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", ErrArchiveLimit
	}
	for _, file := range archive.File {
		if file.Name != "word/document.xml" {
			continue
		}
		body, err := file.Open()
		if err != nil {
			return "", err
		}
		defer body.Close()
		decoder := xml.NewDecoder(io.LimitReader(body, maxExpanded))
		var text strings.Builder
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				return text.String(), nil
			}
			if err != nil {
				return "", ErrArchiveLimit
			}
			if chars, ok := token.(xml.CharData); ok {
				text.Write(chars)
			}
		}
	}
	return "", ErrArchiveLimit
}
