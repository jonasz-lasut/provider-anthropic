/*
Copyright 2026 The provider-anthropic Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package capabilities provides shared infrastructure for Crossplane managed
// resources in this provider.
package capabilities

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
)

// FS is a long-lived content-addressed filesystem cache. It is built once at
// controller startup and shared across reconcile loops.
//
// Stack:
//
//	CacheOnReadFs(
//	  base  = BasePathFs(OsFs, cacheRoot),   // security + recoverability
//	  layer = MemMapFs,                       // performance
//	  cacheTime = 0,                          // no expiry (content-addressed dirs are immutable)
//	)
//
// Writes go directly to base (disk) to avoid the CacheOnReadFs copyFileToLayer
// bug with write-only new files. The union is used for read operations only.
type FS struct {
	base afero.Fs // BasePathFs(OsFs, cacheRoot) — disk writes
	fs   afero.Fs // CacheOnReadFs(base, MemMapFs, 0) — cached reads
}

// New creates the cache root directory on disk and returns an initialised FS.
func New(cacheRoot string) (*FS, error) {
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create cache root %s: %w", cacheRoot, err)
	}
	diskFs := afero.NewBasePathFs(afero.NewOsFs(), cacheRoot)
	memFs := afero.NewMemMapFs()
	return &FS{
		base: diskFs,
		fs:   afero.NewCacheOnReadFs(diskFs, memFs, 0),
	}, nil
}

// CanonicalEncode returns a deterministic byte encoding of data suitable for
// hashing. Keys are sorted alphabetically and length-prefixed to prevent
// ambiguous encodings when keys or values contain delimiter characters.
func CanonicalEncode(data map[string][]byte) []byte {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		fmt.Fprintf(&buf, "%d:%s:%d:", len(k), k, len(data[k]))
		buf.Write(data[k])
		buf.WriteByte('|')
	}
	return buf.Bytes()
}

// Hash returns the full 64-char hex SHA-256 of the canonical encoding of data.
// Use the first 8 characters as the FS directory key (dirKey).
func Hash(data map[string][]byte) string {
	raw := sha256.Sum256(CanonicalEncode(data))
	return hex.EncodeToString(raw[:])
}

// StageFiles writes all entries from data into the content-addressed cache
// directory <namespace>-<name>/<dirKey>/ under the FS root. If the directory
// already exists (cache hit), it returns immediately without re-writing.
// Returns the cache directory path (relative to FS root).
//
// All keys are validated before any write: absolute paths and path-traversal
// sequences (`../`) are rejected. This is a defence-in-depth check —
// BasePathFs also blocks traversal at the filesystem level.
func (f *FS) StageFiles(namespace, name string, data map[string][]byte, dirKey string) (string, error) {
	cacheDir := namespace + "-" + name + "/" + dirKey

	ok, err := afero.Exists(f.fs, cacheDir)
	if err != nil {
		return "", fmt.Errorf("checking cache dir: %w", err)
	}
	if ok {
		return cacheDir, nil
	}

	for key, content := range data {
		clean := filepath.Clean(key)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean+"/", "../") {
			return "", fmt.Errorf("invalid path in data key: %q", key)
		}
		destPath := filepath.ToSlash(filepath.Join(cacheDir, clean))
		if err := f.base.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(destPath), err)
		}
		if err := afero.WriteFile(f.base, destPath, content, 0o600); err != nil {
			return "", fmt.Errorf("write %s: %w", destPath, err)
		}
	}
	return cacheDir, nil
}

// CollectReaders walks cacheDir and returns one io.Reader per file. Each
// reader implements Filename() string, returning the logical path relative to
// cacheDir (e.g. "myskill/SKILL.md"). The Anthropic SDK multipart encoder
// checks Filename() first and uses it verbatim; it falls back to
// path.Base(Name()), which would strip the directory component.
//
// On error, all readers opened so far are closed before returning.
func (f *FS) CollectReaders(cacheDir string) ([]io.Reader, error) {
	var readers []io.Reader
	err := afero.Walk(f.fs, cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		file, err := f.fs.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		logical := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(cacheDir)+"/")
		readers = append(readers, namedReadCloser{ReadCloser: file, filename: logical})
		return nil
	})
	if err != nil {
		for _, r := range readers {
			if rc, ok := r.(io.Closer); ok {
				_ = rc.Close()
			}
		}
		return nil, err
	}
	return readers, nil
}

// namedReadCloser wraps an afero.File and implements Filename() string.
// See CollectReaders for why Filename() is required over Name().
type namedReadCloser struct {
	io.ReadCloser
	filename string
}

func (n namedReadCloser) Filename() string { return n.filename }
