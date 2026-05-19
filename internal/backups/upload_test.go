package backups

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJoinRemoteURLEscapesObjectPath(t *testing.T) {
	got, err := joinRemoteURL("https://dav.example.com/root folder/", "daily/neu drive.zip")
	if err != nil {
		t.Fatalf("joinRemoteURL: %v", err)
	}
	want := "https://dav.example.com/root%20folder/daily/neu%20drive.zip"
	if got != want {
		t.Fatalf("url mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestUploadWebDAVCreatesCollectionsAndPutsArchive(t *testing.T) {
	var sawMKCOL bool
	var sawPUT bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "pass" {
			t.Fatalf("missing basic auth")
		}
		switch r.Method {
		case "MKCOL":
			sawMKCOL = true
			w.WriteHeader(http.StatusCreated)
		case http.MethodPut:
			sawPUT = true
			if r.URL.Path != "/dav/daily/backup.zip" {
				t.Fatalf("put path = %s", r.URL.Path)
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != "zip-bytes" {
				t.Fatalf("body = %q", string(body))
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	svc := &Service{client: server.Client()}
	location, err := svc.uploadWebDAV(context.Background(), Target{
		Kind:           KindWebDAV,
		WebDAVURL:      server.URL + "/dav",
		WebDAVUsername: "user",
	}, targetSecret{WebDAVPassword: "pass"}, "daily/backup.zip", []byte("zip-bytes"))
	if err != nil {
		t.Fatalf("uploadWebDAV: %v", err)
	}
	if !sawMKCOL || !sawPUT {
		t.Fatalf("expected MKCOL and PUT, saw MKCOL=%v PUT=%v", sawMKCOL, sawPUT)
	}
	if !strings.HasSuffix(location, "/dav/daily/backup.zip") {
		t.Fatalf("location = %s", location)
	}
}

func TestUploadS3PathStyleSignsPutObject(t *testing.T) {
	var sawPUT bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s", r.Method)
		}
		sawPUT = true
		if r.URL.Path != "/bucket/prefix/backup.zip" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Amz-Content-Sha256"); got == "" {
			t.Fatalf("missing payload hash")
		}
		if got := r.Header.Get("X-Amz-Date"); got == "" {
			t.Fatalf("missing x-amz-date")
		}
		if got := r.Header.Get("Authorization"); !strings.Contains(got, "Credential=access/") || !strings.Contains(got, "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
			t.Fatalf("authorization header = %s", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "zip-bytes" {
			t.Fatalf("body = %q", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &Service{client: server.Client()}
	location, err := svc.uploadS3(context.Background(), Target{
		Kind:          KindS3,
		S3Endpoint:    server.URL,
		S3Bucket:      "bucket",
		S3Region:      "auto",
		S3AccessKeyID: "access",
		S3PathStyle:   true,
	}, targetSecret{S3SecretAccessKey: "secret"}, "prefix/backup.zip", []byte("zip-bytes"))
	if err != nil {
		t.Fatalf("uploadS3: %v", err)
	}
	if !sawPUT {
		t.Fatalf("expected PUT")
	}
	if !strings.HasSuffix(location, "/bucket/prefix/backup.zip") {
		t.Fatalf("location = %s", location)
	}
}
