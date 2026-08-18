package storage

import (
	"context"
	"strings"
)

// prefixedStorage wraps a Storage so every key lives under a fixed prefix.
// It lets multiple sync profiles share a single bucket: each profile sees
// only its own namespace, including List (used by pull, diff, reset --remote,
// and key verification), so operations on one profile can never touch
// another's files.
type prefixedStorage struct {
	inner  Storage
	prefix string // normalized to always end with "/"
}

// WithPrefix returns a Storage whose keys are transparently namespaced under
// prefix. An empty prefix returns the inner storage unchanged. Leading and
// trailing slashes in prefix are normalized away.
func WithPrefix(inner Storage, prefix string) Storage {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return inner
	}
	return &prefixedStorage{inner: inner, prefix: prefix + "/"}
}

func (p *prefixedStorage) key(key string) string {
	return p.prefix + key
}

func (p *prefixedStorage) Upload(ctx context.Context, key string, data []byte) error {
	return p.inner.Upload(ctx, p.key(key), data)
}

func (p *prefixedStorage) Download(ctx context.Context, key string) ([]byte, error) {
	return p.inner.Download(ctx, p.key(key))
}

func (p *prefixedStorage) Delete(ctx context.Context, key string) error {
	return p.inner.Delete(ctx, p.key(key))
}

func (p *prefixedStorage) DeleteBatch(ctx context.Context, keys []string) error {
	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = p.key(k)
	}
	return p.inner.DeleteBatch(ctx, prefixed)
}

func (p *prefixedStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	objects, err := p.inner.List(ctx, p.key(prefix))
	if err != nil {
		return nil, err
	}
	result := make([]ObjectInfo, 0, len(objects))
	for _, obj := range objects {
		// Keys outside the namespace should be impossible given the List
		// prefix, but never leak them into the profile's view if a backend
		// returns extras.
		if !strings.HasPrefix(obj.Key, p.prefix) {
			continue
		}
		obj.Key = strings.TrimPrefix(obj.Key, p.prefix)
		result = append(result, obj)
	}
	return result, nil
}

func (p *prefixedStorage) Head(ctx context.Context, key string) (*ObjectInfo, error) {
	obj, err := p.inner.Head(ctx, p.key(key))
	if err != nil {
		return nil, err
	}
	if obj != nil {
		trimmed := *obj
		trimmed.Key = strings.TrimPrefix(trimmed.Key, p.prefix)
		return &trimmed, nil
	}
	return obj, nil
}

func (p *prefixedStorage) BucketExists(ctx context.Context) (bool, error) {
	return p.inner.BucketExists(ctx)
}
