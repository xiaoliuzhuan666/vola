package objectstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	BackendDB  = "db"
	BackendCOS = "cos"

	s3ServiceName = "s3"
)

var ErrObjectNotFound = errors.New("object not found")

type Store interface {
	Backend() string
	Enabled() bool
	Key(parts ...string) string
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type COSConfig struct {
	Bucket          string
	Region          string
	Endpoint        string
	SecretID        string
	SecretKey       string
	Prefix          string
	PathStyle       bool
	Client          *http.Client
	RequestTimeFunc func() time.Time
}

type COSStore struct {
	bucket    string
	region    string
	endpoint  *url.URL
	secretID  string
	secretKey string
	prefix    string
	pathStyle bool
	client    *http.Client
	now       func() time.Time
}

func NewCOSStore(cfg COSConfig) (*COSStore, error) {
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("COS bucket is required")
	}
	secretID := strings.TrimSpace(cfg.SecretID)
	if secretID == "" {
		return nil, fmt.Errorf("COS SecretId is required")
	}
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if secretKey == "" {
		return nil, fmt.Errorf("COS SecretKey is required")
	}

	region := strings.TrimSpace(cfg.Region)
	endpointValue := strings.TrimSpace(cfg.Endpoint)
	if endpointValue == "" {
		if region == "" {
			return nil, fmt.Errorf("COS region or endpoint is required")
		}
		endpointValue = "https://cos." + region + ".myqcloud.com"
	}
	if !strings.Contains(endpointValue, "://") {
		endpointValue = "https://" + endpointValue
	}
	endpoint, err := url.Parse(endpointValue)
	if err != nil {
		return nil, fmt.Errorf("invalid COS endpoint: %w", err)
	}
	if endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("COS endpoint must include a scheme and host")
	}
	if region == "" {
		region = regionFromCOSEndpoint(endpoint.Host)
	}
	if region == "" {
		region = "auto"
	}

	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	now := cfg.RequestTimeFunc
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &COSStore{
		bucket:    bucket,
		region:    region,
		endpoint:  endpoint,
		secretID:  secretID,
		secretKey: secretKey,
		prefix:    normalizeKeyPrefix(cfg.Prefix),
		pathStyle: cfg.PathStyle,
		client:    client,
		now:       now,
	}, nil
}

func (s *COSStore) Backend() string {
	return BackendCOS
}

func (s *COSStore) Enabled() bool {
	return s != nil
}

func (s *COSStore) Key(parts ...string) string {
	segments := make([]string, 0, len(parts)+1)
	if s.prefix != "" {
		segments = append(segments, s.prefix)
	}
	for _, part := range parts {
		for _, segment := range strings.Split(strings.Trim(part, "/"), "/") {
			if segment != "" {
				segments = append(segments, segment)
			}
		}
	}
	return strings.Join(segments, "/")
}

func (s *COSStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	location, err := s.objectURL(key)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, location, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", strings.TrimSpace(contentType))
	}
	s.sign(req, data)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("COS put failed with HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *COSStore) Get(ctx context.Context, key string) ([]byte, error) {
	location, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	s.sign(req, nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrObjectNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("COS get failed with HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *COSStore) Delete(ctx context.Context, key string) error {
	location, err := s.objectURL(key)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, location, nil)
	if err != nil {
		return err
	}
	s.sign(req, nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("COS delete failed with HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *COSStore) objectURL(key string) (string, error) {
	if s == nil || s.endpoint == nil {
		return "", fmt.Errorf("COS store is not configured")
	}
	copied := *s.endpoint
	if s.pathStyle {
		copied.Path = joinURLPath(copied.Path, s.bucket, key)
		copied.RawPath = escapePath(copied.Path)
		return copied.String(), nil
	}
	if !strings.HasPrefix(copied.Host, s.bucket+".") {
		copied.Host = s.bucket + "." + copied.Host
	}
	copied.Path = joinURLPath(copied.Path, key)
	copied.RawPath = escapePath(copied.Path)
	return copied.String(), nil
}

func (s *COSStore) sign(req *http.Request, payload []byte) {
	now := s.now().UTC()
	payloadHashBytes := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(payloadHashBytes[:])
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")

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
	scope := strings.Join([]string{shortDate, s.region, s3ServiceName, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(canonicalHashBytes[:]),
	}, "\n")
	signingKey := s3SigningKey(s.secretKey, shortDate, s.region)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.secretID,
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
	return strings.Join(segments, "/")
}

func normalizeKeyPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	segments := make([]string, 0)
	for _, segment := range strings.Split(strings.Trim(prefix, "/"), "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return strings.Join(segments, "/")
}

func regionFromCOSEndpoint(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.Split(host, ":")[0]
	parts := strings.Split(host, ".")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "cos" && parts[i+1] != "" {
			return parts[i+1]
		}
	}
	return ""
}
