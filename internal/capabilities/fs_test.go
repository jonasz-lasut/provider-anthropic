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

package capabilities_test

import (
	"io"
	"testing"

	"github.com/jonasz-lasut/provider-anthropic/internal/capabilities"
)

func newTestFS(t *testing.T) *capabilities.FS {
	t.Helper()
	fs, err := capabilities.New(t.TempDir())
	if err != nil {
		t.Fatalf("capabilities.New: %v", err)
	}
	return fs
}

func TestCanonicalEncode_Deterministic(t *testing.T) {
	data1 := map[string][]byte{
		"b.txt": []byte("beta"),
		"a.txt": []byte("alpha"),
	}
	data2 := map[string][]byte{
		"a.txt": []byte("alpha"),
		"b.txt": []byte("beta"),
	}
	if string(capabilities.CanonicalEncode(data1)) != string(capabilities.CanonicalEncode(data2)) {
		t.Error("CanonicalEncode is not deterministic across key orderings")
	}
}

func TestCanonicalEncode_SensitiveToContent(t *testing.T) {
	data1 := map[string][]byte{"k": []byte("v1")}
	data2 := map[string][]byte{"k": []byte("v2")}
	if string(capabilities.CanonicalEncode(data1)) == string(capabilities.CanonicalEncode(data2)) {
		t.Error("CanonicalEncode should differ when content differs")
	}
}

func TestCanonicalEncode_KeyBoundary(t *testing.T) {
	// Verify length-prefixing prevents {"ab":"cd"} == {"a":"bcd"}
	data1 := map[string][]byte{"ab": []byte("cd")}
	data2 := map[string][]byte{"a": []byte("bcd")}
	if string(capabilities.CanonicalEncode(data1)) == string(capabilities.CanonicalEncode(data2)) {
		t.Error("CanonicalEncode must distinguish different key/value boundaries")
	}
}

func TestHash(t *testing.T) {
	data := map[string][]byte{"myskill/SKILL.md": []byte("# Hello")}
	h1 := capabilities.Hash(data)
	h2 := capabilities.Hash(data)
	if h1 != h2 {
		t.Error("Hash not deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars: %s", len(h1), h1)
	}
}

func TestStageFiles_CacheHit(t *testing.T) {
	fs := newTestFS(t)
	data := map[string][]byte{"myskill/SKILL.md": []byte("# My Skill\n")}
	fullHex := capabilities.Hash(data)
	dirKey := fullHex[:8]

	cacheDir, err := fs.StageFiles("default", "my-skill", data, dirKey)
	if err != nil {
		t.Fatalf("first StageFiles: %v", err)
	}
	cacheDir2, err := fs.StageFiles("default", "my-skill", data, dirKey)
	if err != nil {
		t.Fatalf("second StageFiles (cache hit): %v", err)
	}
	if cacheDir != cacheDir2 {
		t.Errorf("cache dirs differ: %q vs %q", cacheDir, cacheDir2)
	}
}

func TestStageFiles_InvalidPath(t *testing.T) {
	fs := newTestFS(t)
	bad := map[string][]byte{"../etc/passwd": []byte("root:x:0:0")}
	_, err := fs.StageFiles("default", "my-skill", bad, "deadbeef")
	if err == nil {
		t.Error("expected error for path-traversal key, got nil")
	}
}

func TestStageFiles_AbsolutePath(t *testing.T) {
	fs := newTestFS(t)
	bad := map[string][]byte{"/etc/passwd": []byte("root")}
	_, err := fs.StageFiles("default", "my-skill", bad, "deadbeef")
	if err == nil {
		t.Error("expected error for absolute path key, got nil")
	}
}

func TestCollectReaders_LogicalNames(t *testing.T) {
	fs := newTestFS(t)
	data := map[string][]byte{
		"myskill/SKILL.md":  []byte("# My Skill\n"),
		"myskill/helper.py": []byte("print('hi')\n"),
	}
	fullHex := capabilities.Hash(data)
	dirKey := fullHex[:8]

	cacheDir, err := fs.StageFiles("default", "my-skill", data, dirKey)
	if err != nil {
		t.Fatalf("StageFiles: %v", err)
	}
	readers, err := fs.CollectReaders(cacheDir)
	if err != nil {
		t.Fatalf("CollectReaders: %v", err)
	}
	if len(readers) != 2 {
		t.Fatalf("expected 2 readers, got %d", len(readers))
	}

	// Each reader must implement Filename() so the SDK multipart encoder
	// preserves the full path (e.g. "myskill/SKILL.md", not "SKILL.md").
	names := map[string]bool{}
	for _, r := range readers {
		type namer interface{ Filename() string }
		fn, ok := r.(namer)
		if !ok {
			t.Fatal("reader does not implement Filename() string")
		}
		names[fn.Filename()] = true
		content, _ := io.ReadAll(r)
		if len(content) == 0 {
			t.Errorf("reader %s has empty content", fn.Filename())
		}
	}
	if !names["myskill/SKILL.md"] {
		t.Error("expected myskill/SKILL.md in reader names")
	}
	if !names["myskill/helper.py"] {
		t.Error("expected myskill/helper.py in reader names")
	}
}
