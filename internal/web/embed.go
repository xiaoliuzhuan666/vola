package web

import (
	"embed"
	"encoding/json"
	"html"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded frontend.
// For SPA routing, it falls back to index.html for non-file paths.
func Handler() http.Handler {
	dist, _ := fs.Sub(distFS, "dist")
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean path
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file in the embedded FS
		f, err := dist.(fs.ReadFileFS).ReadFile(path)
		if err != nil {
			// File not found — serve index.html for SPA client-side routing
			indexFile, err := fs.ReadFile(dist, "index.html")
			if err != nil {
				http.Error(w, "frontend not built", http.StatusNotFound)
				return
			}
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(renderIndexHTML(indexFile, seoForPath(r.URL.Path)))
			return
		}
		if path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(renderIndexHTML(f, seoForPath(r.URL.Path)))
			return
		}
		_ = f

		setFrontendCacheHeaders(w, path)
		// File exists — let the standard file server handle it
		// (it sets correct Content-Type, caching headers, etc.)
		fileServer.ServeHTTP(w, r)
	})
}

type pageSEO struct {
	Title       string
	Description string
	URL         string
	Robots      string
}

const (
	siteURL            = "https://www.vola.cn"
	productName        = "Vola"
	defaultTitle       = "Personal data hub for AI agents — Vola"
	defaultDescription = "Vola is a personal data hub for AI agents, connecting profile, memory, projects, conversations, skills, and vault access."
)

var (
	titleTagRe             = regexp.MustCompile(`(?is)<title>.*?</title>`)
	descriptionMetaRe      = regexp.MustCompile(`(?is)<meta\s+name="description"\s+content="[^"]*"\s*/?>`)
	robotsMetaRe           = regexp.MustCompile(`(?is)<meta\s+name="robots"\s+content="[^"]*"\s*/?>`)
	canonicalLinkRe        = regexp.MustCompile(`(?is)<link\s+rel="canonical"\s+href="[^"]*"\s*/?>`)
	ogTitleMetaRe          = regexp.MustCompile(`(?is)<meta\s+property="og:title"\s+content="[^"]*"\s*/?>`)
	ogDescriptionMetaRe    = regexp.MustCompile(`(?is)<meta\s+property="og:description"\s+content="[^"]*"\s*/?>`)
	ogURLMetaRe            = regexp.MustCompile(`(?is)<meta\s+property="og:url"\s+content="[^"]*"\s*/?>`)
	twitterTitleMetaRe     = regexp.MustCompile(`(?is)<meta\s+name="twitter:title"\s+content="[^"]*"\s*/?>`)
	twitterDescriptionRe   = regexp.MustCompile(`(?is)<meta\s+name="twitter:description"\s+content="[^"]*"\s*/?>`)
	structuredDataScriptRe = regexp.MustCompile(`(?is)<script\s+type="application/ld\+json"\s+id="structured-data">.*?</script>`)
)

func seoForPath(rawPath string) pageSEO {
	cleanPath := "/" + strings.Trim(strings.TrimSpace(rawPath), "/")
	if cleanPath == "//" {
		cleanPath = "/"
	}
	seo := pageSEO{
		Title:       defaultTitle,
		Description: defaultDescription,
		URL:         siteURL + cleanPath,
		Robots:      "index, follow",
	}
	switch {
	case cleanPath == "/":
		seo.URL = siteURL + "/"
	case cleanPath == "/pricing":
		seo.Title = productName
		seo.Description = defaultDescription
		seo.Robots = "noindex, nofollow"
	case cleanPath == "/integrations":
		seo.Title = "Integrations — " + productName
		seo.Description = "Connect Vola to Claude, ChatGPT, Cursor, Windsurf, CLI agents, MCP, and REST API."
	case cleanPath == "/docs":
		seo.Title = "Docs — " + productName
		seo.Description = "Setup guides for connecting Vola to Claude, ChatGPT, coding editors, CLI agents, and custom MCP clients."
	case strings.HasPrefix(cleanPath, "/integrations/"):
		name := integrationName(strings.TrimPrefix(cleanPath, "/integrations/"))
		seo.Title = name + " Integration — " + productName
		seo.Description = "Learn how to connect " + name + " to Vola so it can use shared memory, files, and skills."
	case strings.HasPrefix(cleanPath, "/guides/"):
		name := integrationName(strings.TrimPrefix(cleanPath, "/guides/"))
		seo.Title = name + " Setup Guide — " + productName
		seo.Description = "Follow the Vola setup guide for " + name + ", including copyable URLs, authorization steps, and a test prompt."
	case cleanPath == "/privacy":
		seo.Title = "Privacy — " + productName
		seo.Description = "How Vola handles AI memory, files, credentials, connections, exports, and deletion."
	case cleanPath == "/terms":
		seo.Title = "Terms — " + productName
		seo.Description = "Terms for using Vola and connecting AI tools to your memory, files, and skills."
	case cleanPath == "/login" || cleanPath == "/signup" || isPrivateAppPath(cleanPath):
		seo.Title = productName
		seo.Description = defaultDescription
		seo.Robots = "noindex, nofollow"
	}
	return seo
}

func integrationName(key string) string {
	switch strings.ToLower(strings.Trim(key, "/")) {
	case "claude", "cloud":
		return "Claude"
	case "chatgpt", "openai", "apps":
		return "ChatGPT Apps"
	case "editors", "cursor", "windsurf":
		return "Cursor and Windsurf"
	case "cli", "codex", "claude-code", "gemini":
		return "CLI Agents"
	case "api", "mcp", "sdk":
		return "MCP and REST API"
	default:
		return "AI Tool"
	}
}

func isPrivateAppPath(path string) bool {
	privatePrefixes := []string{
		"/api/", "/oauth/", "/settings/", "/data/", "/imports/", "/sync/", "/billing/",
		"/onboarding", "/setup/", "/cli", "/mcp", "/gpt/", "/stripe/",
	}
	for _, prefix := range privatePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return path == "/plan"
}

func renderIndexHTML(indexFile []byte, seo pageSEO) []byte {
	htmlText := string(indexFile)
	title := html.EscapeString(seo.Title)
	description := html.EscapeString(seo.Description)
	urlValue := html.EscapeString(seo.URL)
	robots := html.EscapeString(seo.Robots)

	htmlText = titleTagRe.ReplaceAllString(htmlText, "<title>"+title+"</title>")
	htmlText = descriptionMetaRe.ReplaceAllString(htmlText, `<meta name="description" content="`+description+`" />`)
	htmlText = robotsMetaRe.ReplaceAllString(htmlText, `<meta name="robots" content="`+robots+`" />`)
	htmlText = canonicalLinkRe.ReplaceAllString(htmlText, `<link rel="canonical" href="`+urlValue+`" />`)
	htmlText = ogTitleMetaRe.ReplaceAllString(htmlText, `<meta property="og:title" content="`+title+`" />`)
	htmlText = ogDescriptionMetaRe.ReplaceAllString(htmlText, `<meta property="og:description" content="`+description+`" />`)
	htmlText = ogURLMetaRe.ReplaceAllString(htmlText, `<meta property="og:url" content="`+urlValue+`" />`)
	htmlText = twitterTitleMetaRe.ReplaceAllString(htmlText, `<meta name="twitter:title" content="`+title+`" />`)
	htmlText = twitterDescriptionRe.ReplaceAllString(htmlText, `<meta name="twitter:description" content="`+description+`" />`)
	htmlText = structuredDataScriptRe.ReplaceAllString(htmlText, structuredDataScript(seo))
	return []byte(htmlText)
}

func structuredDataScript(seo pageSEO) string {
	payload := map[string]interface{}{
		"@context":            "https://schema.org",
		"@type":               "SoftwareApplication",
		"name":                productName,
		"applicationCategory": "ProductivityApplication",
		"operatingSystem":     "Web",
		"url":                 seo.URL,
		"description":         seo.Description,
		"logo":                siteURL + "/vola-app-icon.png",
		"image":               siteURL + "/vola-social.svg",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"@context":"https://schema.org","@type":"SoftwareApplication","name":"Vola"}`)
	}
	return `<script type="application/ld+json" id="structured-data">` + string(body) + `</script>`
}

func setFrontendCacheHeaders(w http.ResponseWriter, path string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".js", ".css":
		w.Header().Set("Cache-Control", "no-cache")
	}
	if contentType := mime.TypeByExtension(ext); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
}

// DevProxy returns true if the VOLA_DEV environment variable is set,
// indicating the frontend dev server should be used instead of embedded assets.
func DevProxy() bool {
	return os.Getenv("VOLA_DEV") != ""
}
