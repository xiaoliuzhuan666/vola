package objectstore

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCOSStoreVirtualHostedRequests(t *testing.T) {
	transport := &captureTransport{statusCode: http.StatusOK}
	store, err := NewCOSStore(COSConfig{
		Bucket:    "demo-1250000000",
		Region:    "ap-guangzhou",
		Endpoint:  "https://cos.ap-guangzhou.myqcloud.com",
		SecretID:  "secret-id",
		SecretKey: "secret-key",
		Prefix:    "neudrive",
		Client:    &http.Client{Transport: transport},
		RequestTimeFunc: func() time.Time {
			return time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewCOSStore: %v", err)
	}

	if err := store.Put(context.Background(), store.Key("users", "u1", "hello world.txt"), []byte("hello"), "text/plain"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if transport.request.Host != "demo-1250000000.cos.ap-guangzhou.myqcloud.com" {
		t.Fatalf("host = %q", transport.request.Host)
	}
	if transport.request.URL.EscapedPath() != "/neudrive/users/u1/hello%20world.txt" {
		t.Fatalf("path = %q", transport.request.URL.EscapedPath())
	}
	if !strings.Contains(transport.request.Header.Get("Authorization"), "Credential=secret-id/20260514/ap-guangzhou/s3/aws4_request") {
		t.Fatalf("authorization = %q", transport.request.Header.Get("Authorization"))
	}
}

func TestCOSStorePathStyleRequests(t *testing.T) {
	transport := &captureTransport{statusCode: http.StatusOK}
	store, err := NewCOSStore(COSConfig{
		Bucket:    "demo-1250000000",
		Region:    "ap-guangzhou",
		Endpoint:  "https://cos.ap-guangzhou.myqcloud.com",
		SecretID:  "secret-id",
		SecretKey: "secret-key",
		Prefix:    "neudrive",
		PathStyle: true,
		Client:    &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("NewCOSStore: %v", err)
	}

	if err := store.Delete(context.Background(), store.Key("entry")); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if transport.request.URL.EscapedPath() != "/demo-1250000000/neudrive/entry" {
		t.Fatalf("path = %q", transport.request.URL.EscapedPath())
	}
}

func TestCOSStoreGetNotFound(t *testing.T) {
	transport := &captureTransport{statusCode: http.StatusNotFound}
	store, err := NewCOSStore(COSConfig{
		Bucket:    "demo-1250000000",
		Region:    "ap-guangzhou",
		Endpoint:  "https://cos.ap-guangzhou.myqcloud.com",
		SecretID:  "secret-id",
		SecretKey: "secret-key",
		Client:    &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("NewCOSStore: %v", err)
	}

	_, err = store.Get(context.Background(), "missing")
	if err != ErrObjectNotFound {
		t.Fatalf("Get err = %v, want ErrObjectNotFound", err)
	}
}

type captureTransport struct {
	request    *http.Request
	statusCode int
	body       string
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copied := req.Clone(req.Context())
	copied.Body = nil
	t.request = copied
	body := t.body
	if body == "" {
		body = "ok"
	}
	return &http.Response{
		StatusCode: t.statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    req,
	}, nil
}
