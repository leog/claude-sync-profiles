package storage

import (
	"context"
	"reflect"
	"testing"
)

// fakeStorage records the keys it is called with and serves canned objects.
type fakeStorage struct {
	uploads   map[string][]byte
	deleted   []string
	listed    []ObjectInfo
	lastList  string
	lastHead  string
	headReply *ObjectInfo
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{uploads: make(map[string][]byte)}
}

func (f *fakeStorage) Upload(ctx context.Context, key string, data []byte) error {
	f.uploads[key] = data
	return nil
}

func (f *fakeStorage) Download(ctx context.Context, key string) ([]byte, error) {
	return f.uploads[key], nil
}

func (f *fakeStorage) Delete(ctx context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *fakeStorage) DeleteBatch(ctx context.Context, keys []string) error {
	f.deleted = append(f.deleted, keys...)
	return nil
}

func (f *fakeStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	f.lastList = prefix
	return f.listed, nil
}

func (f *fakeStorage) Head(ctx context.Context, key string) (*ObjectInfo, error) {
	f.lastHead = key
	return f.headReply, nil
}

func (f *fakeStorage) BucketExists(ctx context.Context) (bool, error) {
	return true, nil
}

func TestWithPrefixEmptyReturnsInner(t *testing.T) {
	inner := newFakeStorage()
	if got := WithPrefix(inner, ""); got != Storage(inner) {
		t.Error("WithPrefix with empty prefix should return the inner storage unchanged")
	}
	if got := WithPrefix(inner, "///"); got != Storage(inner) {
		t.Error("WithPrefix with slash-only prefix should return the inner storage unchanged")
	}
}

func TestWithPrefixNamespacesKeys(t *testing.T) {
	ctx := context.Background()
	inner := newFakeStorage()
	store := WithPrefix(inner, "/personal/")

	if err := store.Upload(ctx, "CLAUDE.md.age", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, ok := inner.uploads["personal/CLAUDE.md.age"]; !ok {
		t.Errorf("Upload should prefix keys, got %v", inner.uploads)
	}

	data, err := store.Download(ctx, "CLAUDE.md.age")
	if err != nil || string(data) != "x" {
		t.Errorf("Download should read the prefixed key, got %q, %v", data, err)
	}

	if err := store.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteBatch(ctx, []string{"b", "c"}); err != nil {
		t.Fatal(err)
	}
	wantDeleted := []string{"personal/a", "personal/b", "personal/c"}
	if !reflect.DeepEqual(inner.deleted, wantDeleted) {
		t.Errorf("Delete/DeleteBatch keys = %v, want %v", inner.deleted, wantDeleted)
	}

	inner.headReply = &ObjectInfo{Key: "personal/settings.json.age", Size: 5}
	obj, err := store.Head(ctx, "settings.json.age")
	if err != nil {
		t.Fatal(err)
	}
	if inner.lastHead != "personal/settings.json.age" {
		t.Errorf("Head should query the prefixed key, got %q", inner.lastHead)
	}
	if obj.Key != "settings.json.age" {
		t.Errorf("Head should strip the prefix from the result key, got %q", obj.Key)
	}
}

func TestWithPrefixList(t *testing.T) {
	ctx := context.Background()
	inner := newFakeStorage()
	inner.listed = []ObjectInfo{
		{Key: "personal/projects/a.jsonl.age"},
		{Key: "personal/_external/mcp-servers.json.age"},
		{Key: "work/projects/b.jsonl.age"}, // outside the namespace: must not leak
	}
	store := WithPrefix(inner, "personal")

	objects, err := store.List(ctx, "projects/")
	if err != nil {
		t.Fatal(err)
	}
	if inner.lastList != "personal/projects/" {
		t.Errorf("List should prepend the namespace to the prefix, got %q", inner.lastList)
	}

	var keys []string
	for _, obj := range objects {
		keys = append(keys, obj.Key)
	}
	want := []string{"projects/a.jsonl.age", "_external/mcp-servers.json.age"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("List keys = %v, want %v", keys, want)
	}
}
