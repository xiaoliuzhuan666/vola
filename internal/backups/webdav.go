package backups

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (s *Service) uploadWebDAV(ctx context.Context, target Target, secret targetSecret, objectName string, data []byte) (string, error) {
	location, err := joinRemoteURL(target.WebDAVURL, objectName)
	if err != nil {
		return "", err
	}
	if err := s.ensureWebDAVCollections(ctx, target, secret, objectName); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, location, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/zip")
	if target.WebDAVUsername != "" || secret.WebDAVPassword != "" {
		req.SetBasicAuth(target.WebDAVUsername, secret.WebDAVPassword)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("WebDAV upload failed with HTTP %d", resp.StatusCode)
	}
	return location, nil
}

func (s *Service) deleteWebDAV(ctx context.Context, target Target, secret targetSecret, objectName string) error {
	location, err := joinRemoteURL(target.WebDAVURL, objectName)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, location, nil)
	if err != nil {
		return err
	}
	if target.WebDAVUsername != "" || secret.WebDAVPassword != "" {
		req.SetBasicAuth(target.WebDAVUsername, secret.WebDAVPassword)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("WebDAV delete failed with HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) ensureWebDAVCollections(ctx context.Context, target Target, secret targetSecret, objectName string) error {
	parts := strings.Split(strings.Trim(objectName, "/"), "/")
	if len(parts) <= 1 {
		return nil
	}
	prefixParts := parts[:len(parts)-1]
	for idx := range prefixParts {
		collection := strings.Join(prefixParts[:idx+1], "/")
		location, err := joinRemoteURL(target.WebDAVURL, collection)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, "MKCOL", location, nil)
		if err != nil {
			return err
		}
		if target.WebDAVUsername != "" || secret.WebDAVPassword != "" {
			req.SetBasicAuth(target.WebDAVUsername, secret.WebDAVPassword)
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		status := resp.StatusCode
		_ = resp.Body.Close()
		if status >= 200 && status < 300 {
			continue
		}
		if status == http.StatusMethodNotAllowed || status == http.StatusConflict {
			continue
		}
		return fmt.Errorf("WebDAV collection creation failed with HTTP %d", status)
	}
	return nil
}

func joinRemoteURL(baseURL, objectName string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	rawParts := make([]string, 0)
	escapedParts := make([]string, 0)
	for _, part := range strings.Split(strings.Trim(objectName, "/"), "/") {
		if part == "" {
			continue
		}
		rawParts = append(rawParts, part)
		escapedParts = append(escapedParts, url.PathEscape(part))
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	rawPath := strings.TrimRight(parsed.EscapedPath(), "/")
	if len(rawParts) > 0 {
		basePath += "/" + strings.Join(rawParts, "/")
		rawPath += "/" + strings.Join(escapedParts, "/")
	}
	if basePath == "" {
		basePath = "/"
	}
	if rawPath == "" {
		rawPath = "/"
	}
	parsed.Path = basePath
	parsed.RawPath = rawPath
	return parsed.String(), nil
}
