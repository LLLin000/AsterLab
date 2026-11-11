package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func tempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewLinkUsesRelativePath(t *testing.T) {
	root := tempRoot(t)
	path := filepath.Join(root, "huvec", "d1", "image_01.tif")
	writeFile(t, path, "fake-tiff-bytes")
	asset, err := NewLink("project-data", root, path, Link{RunID: "run-1", TaskKey: "t1"}, "tester", true)
	if err != nil {
		t.Fatal(err)
	}
	if asset.RelativePath != "huvec/d1/image_01.tif" {
		t.Fatalf("relative path = %q", asset.RelativePath)
	}
	if asset.RootID != "project-data" {
		t.Fatalf("root = %q", asset.RootID)
	}
	if asset.MediaType != MediaImage {
		t.Fatalf("media = %q", asset.MediaType)
	}
	if asset.SHA256 == "" {
		t.Fatal("hash missing for a small file")
	}
	if asset.Availability != Available {
		t.Fatalf("availability = %q", asset.Availability)
	}
}

func TestAssetIDUsesStableLogicalRootAndRelativePath(t *testing.T) {
	roots := []string{tempRoot(t), tempRoot(t)}
	ids := make([]string, len(roots))
	for i, root := range roots {
		path := filepath.Join(root, "huvec", "image.tif")
		writeFile(t, path, "same bytes")
		asset, err := NewLink("project-data", root, path, Link{RunID: "run-1", TaskKey: "t1"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = asset.ID
	}
	if ids[0] != ids[1] || len(ids[0]) != len("asset-")+32 {
		t.Fatalf("unstable asset ID: %q vs %q", ids[0], ids[1])
	}
}

func TestNewLinkRejectsPathEscapingRoot(t *testing.T) {
	root := tempRoot(t)
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	writeFile(t, outside, "x")
	if _, err := NewLink("project-data", root, outside, Link{RunID: "r", TaskKey: "t1"}, "", false); err == nil {
		t.Fatal("accepted a path outside the root")
	}
}

func TestRootsResolveAndRejectTraversal(t *testing.T) {
	root := tempRoot(t)
	file := filepath.Join(root, "huvec", "d1", "image_01.tif")
	writeFile(t, file, "x")
	r := Roots{"project-data": root}
	a := Asset{ID: "a1", RootID: "project-data", RelativePath: "huvec/d1/image_01.tif"}
	path, err := r.Resolve(a)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "huvec", "d1", "image_01.tif") {
		t.Fatalf("path = %q", path)
	}
	// Traversal must be refused, not silently joined.
	bad := Asset{ID: "a2", RootID: "project-data", RelativePath: "../../etc/passwd"}
	if _, err := r.Resolve(bad); err == nil {
		t.Fatal("traversal path accepted")
	}
	// Unknown root is an honest error, not a guessed path.
	if _, err := (Roots{}).Resolve(a); err == nil {
		t.Fatal("unknown root accepted")
	}
}

func TestVerifyDetectsChangedSize(t *testing.T) {
	root := tempRoot(t)
	path := filepath.Join(root, "data.csv")
	writeFile(t, path, "a,b\n1,2\n")
	size, mod, hash, avail := Describe(path, true)
	if avail != Available {
		t.Fatal("file not available")
	}
	a := Asset{Size: size, ModTime: mod, SHA256: hash}
	ok, reason := Verify(a, path, true)
	if !ok {
		t.Fatalf("unchanged file rejected: %s", reason)
	}
	// Replace content: size changes.
	writeFile(t, path, "a,b,c\n1,2,3\n4,5,6\n")
	ok, reason = Verify(a, path, true)
	if ok {
		t.Fatal("changed file accepted")
	}
	if reason == "" {
		t.Fatal("no reason for rejection")
	}
}

func TestUnavailableKeepsLink(t *testing.T) {
	root := tempRoot(t)
	path := filepath.Join(root, "gone.tif")
	writeFile(t, path, "x")
	asset, err := NewLink("project-data", root, path, Link{RunID: "r", TaskKey: "t1"}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	// Delete the file: link must survive with Unavailable, not disappear.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	size, _, _, avail := Describe(path, false)
	if avail != Unavailable {
		t.Fatal("missing file reported available")
	}
	if size != 0 {
		t.Fatalf("size = %d for missing file", size)
	}
	if asset.ID == "" || asset.RelativePath == "" {
		t.Fatal("asset identity lost")
	}
	ok, reason := Verify(asset, path, false)
	if ok {
		t.Fatal("missing file verified as present")
	}
	if reason == "" {
		t.Fatal("missing file rejected without reason")
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]MediaType{
		"a.TIF": MediaImage, "b.csv": MediaTable, "c.pdf": MediaDocument,
		"d.zip": MediaBinary, "e.xyz": MediaUnknown,
	}
	for name, want := range cases {
		if got := Classify(name); got != want {
			t.Fatalf("Classify(%q) = %q, want %q", name, got, want)
		}
	}
}
