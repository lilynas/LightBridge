package modules

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ChecksumsFilename = "checksums.txt"

type ChecksumEntry struct {
	Algorithm string
	HexDigest string
	Path      string
}

func ParseChecksums(content []byte) ([]ChecksumEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	var entries []ChecksumEntry
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid checksum line %d", lineNo)
		}
		algorithm := strings.ToLower(fields[0])
		if algorithm != "sha256" {
			return nil, fmt.Errorf("unsupported checksum algorithm %q on line %d", fields[0], lineNo)
		}
		digest := strings.ToLower(fields[1])
		if len(digest) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid sha256 digest length on line %d", lineNo)
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return nil, fmt.Errorf("invalid sha256 digest on line %d: %w", lineNo, err)
		}
		if err := validateRelativePath("checksum path", fields[2]); err != nil {
			return nil, err
		}
		entries = append(entries, ChecksumEntry{
			Algorithm: algorithm,
			HexDigest: digest,
			Path:      filepath.ToSlash(filepath.Clean(fields[2])),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("checksums file is empty")
	}
	return entries, nil
}

func LoadChecksumsFile(path string) ([]ChecksumEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseChecksums(content)
}

func VerifyChecksums(root string, entries []ChecksumEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no checksum entries")
	}
	for _, entry := range entries {
		if entry.Algorithm != "sha256" {
			return fmt.Errorf("unsupported checksum algorithm %q for %s", entry.Algorithm, entry.Path)
		}
		path := filepath.Join(root, filepath.FromSlash(entry.Path))
		sum, err := sha256File(path)
		if err != nil {
			return fmt.Errorf("checksum %s: %w", entry.Path, err)
		}
		if sum != entry.HexDigest {
			return fmt.Errorf("checksum mismatch for %s", entry.Path)
		}
	}
	return nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
