package syncengine

import (
	"context"
	"errors"
)

// ErrAuthExpired indicates OAuth2 or cloud backend credentials have expired or
// been revoked and require re-authorization.
var ErrAuthExpired = errors.New("sync: authorization expired or invalid")

// RemoteStore abstracts remote Vault object storage across WebDAV, OneDrive,
// and Google Drive backends for Engine.
type RemoteStore interface {
	Get(ctx context.Context, relPath string) ([]byte, error)
	GetWithETag(ctx context.Context, relPath string) (data []byte, etag string, err error)
	GetVerified(ctx context.Context, relPath, wantSHA256 string) ([]byte, error)
	Put(ctx context.Context, relPath string, data []byte, contentType string) error
	PutIfMatch(ctx context.Context, relPath string, data []byte, contentType, ifMatchETag string) (newETag string, err error)
	Delete(ctx context.Context, relPath string) error
	TestConnection(ctx context.Context) error
	RequestCount() int64
}
