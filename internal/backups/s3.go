package backups

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const s3ServiceName = "s3"

func (s *Service) uploadS3(ctx context.Context, target Target, secret targetSecret, objectName string, data []byte) (string, error) {
	secretKey := strings.TrimSpace(secret.S3SecretAccessKey)
	if secretKey == "" {
		return "", fmt.Errorf("S3-compatible backup requires a saved secret access key")
	}
	region := strings.TrimSpace(target.S3Region)
	if region == "" {
		region = "auto"
	}
	location, err := s3ObjectURL(target, objectName)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, location, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/zip")
	signS3Request(req, data, strings.TrimSpace(target.S3AccessKeyID), secretKey, region, time.Now().UTC())
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("S3-compatible upload failed with HTTP %d", resp.StatusCode)
	}
	return location, nil
}

func (s *Service) deleteS3(ctx context.Context, target Target, secret targetSecret, objectName string) error {
	secretKey := strings.TrimSpace(secret.S3SecretAccessKey)
	if secretKey == "" {
		return fmt.Errorf("S3-compatible backup requires a saved secret access key")
	}
	region := strings.TrimSpace(target.S3Region)
	if region == "" {
		region = "auto"
	}
	location, err := s3ObjectURL(target, objectName)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, location, nil)
	if err != nil {
		return err
	}
	signS3Request(req, nil, strings.TrimSpace(target.S3AccessKeyID), secretKey, region, time.Now().UTC())
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("S3-compatible delete failed with HTTP %d", resp.StatusCode)
	}
	return nil
}

func s3ObjectURL(target Target, objectName string) (string, error) {
	endpoint, err := url.Parse(strings.TrimSpace(target.S3Endpoint))
	if err != nil {
		return "", err
	}
	if endpoint.Scheme == "" || endpoint.Host == "" {
		return "", fmt.Errorf("S3 endpoint must be a valid URL")
	}
	if target.S3PathStyle {
		endpoint.Path = joinURLPath(endpoint.Path, target.S3Bucket, objectName)
		endpoint.RawPath = escapePath(endpoint.Path)
		return endpoint.String(), nil
	}
	endpoint.Host = strings.TrimSpace(target.S3Bucket) + "." + endpoint.Host
	endpoint.Path = joinURLPath(endpoint.Path, objectName)
	endpoint.RawPath = escapePath(endpoint.Path)
	return endpoint.String(), nil
}

func signS3Request(req *http.Request, payload []byte, accessKeyID, secretKey, region string, now time.Time) {
	payloadHashBytes := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(payloadHashBytes[:])
	amzDate := now.UTC().Format("20060102T150405Z")
	shortDate := now.UTC().Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Host = req.URL.Host

	canonicalHeaders, signedHeaders := canonicalS3Headers(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		s3CanonicalURI(req.URL),
		canonicalQuery(req.URL),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	canonicalHashBytes := sha256.Sum256([]byte(canonicalRequest))
	scope := strings.Join([]string{shortDate, region, s3ServiceName, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(canonicalHashBytes[:]),
	}, "\n")
	signingKey := s3SigningKey(secretKey, shortDate, region)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKeyID,
		scope,
		signedHeaders,
		signature,
	))
}

func canonicalS3Headers(req *http.Request) (string, string) {
	headers := map[string]string{
		"host":                 strings.ToLower(req.URL.Host),
		"x-amz-content-sha256": strings.TrimSpace(req.Header.Get("X-Amz-Content-Sha256")),
		"x-amz-date":           strings.TrimSpace(req.Header.Get("X-Amz-Date")),
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+":"+headers[key])
	}
	return strings.Join(lines, "\n") + "\n", strings.Join(keys, ";")
}

func canonicalQuery(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	values := u.Query()
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0)
	for _, key := range keys {
		vals := values[key]
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func s3CanonicalURI(u *url.URL) string {
	if u.RawPath != "" {
		return u.RawPath
	}
	return escapePath(u.Path)
}

func s3SigningKey(secretKey, shortDate, region string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secretKey), shortDate)
	regionKey := hmacSHA256(dateKey, region)
	serviceKey := hmacSHA256(regionKey, s3ServiceName)
	return hmacSHA256(serviceKey, "aws4_request")
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func joinURLPath(base string, parts ...string) string {
	all := make([]string, 0, len(parts)+1)
	if trimmed := strings.Trim(base, "/"); trimmed != "" {
		all = append(all, strings.Split(trimmed, "/")...)
	}
	for _, part := range parts {
		for _, segment := range strings.Split(strings.Trim(part, "/"), "/") {
			if segment != "" {
				all = append(all, segment)
			}
		}
	}
	if len(all) == 0 {
		return "/"
	}
	return "/" + strings.Join(all, "/")
}

func escapePath(value string) string {
	segments := strings.Split(value, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	escaped := strings.Join(segments, "/")
	if !strings.HasPrefix(escaped, "/") {
		escaped = "/" + escaped
	}
	return escaped
}
