package web

import (
	"strings"
	"testing"
)

func TestSEOForPublicRoutes(t *testing.T) {
	cases := []struct {
		path      string
		title     string
		canonical string
		robots    string
	}{
		{"/", defaultTitle, "https://www.vola.ai/", "index, follow"},
		{"/pricing", "Vola", "https://www.vola.ai/pricing", "noindex, nofollow"},
		{"/guides/chatgpt", "ChatGPT Apps Setup Guide — Vola", "https://www.vola.ai/guides/chatgpt", "index, follow"},
		{"/settings/profile", "Vola", "https://www.vola.ai/settings/profile", "noindex, nofollow"},
	}

	for _, tc := range cases {
		seo := seoForPath(tc.path)
		if seo.Title != tc.title {
			t.Fatalf("%s title = %q, want %q", tc.path, seo.Title, tc.title)
		}
		if seo.URL != tc.canonical {
			t.Fatalf("%s url = %q, want %q", tc.path, seo.URL, tc.canonical)
		}
		if seo.Robots != tc.robots {
			t.Fatalf("%s robots = %q, want %q", tc.path, seo.Robots, tc.robots)
		}
	}
}

func TestRenderIndexHTMLInjectsRouteSEO(t *testing.T) {
	index := []byte(`<!doctype html><html><head>
<title>Old</title>
<meta name="description" content="old" />
<meta name="robots" content="index, follow" />
<link rel="canonical" href="https://www.vola.ai/" />
<meta property="og:title" content="old" />
<meta property="og:description" content="old" />
<meta property="og:url" content="https://www.vola.ai/" />
<meta name="twitter:title" content="old" />
<meta name="twitter:description" content="old" />
<script type="application/ld+json" id="structured-data">{}</script>
</head><body></body></html>`)

	rendered := string(renderIndexHTML(index, seoForPath("/integrations/claude")))
	mustContain := []string{
		"<title>Claude Integration — Vola</title>",
		`<meta name="description" content="Learn how to connect Claude to Vola so it can use shared memory, files, and skills." />`,
		`<link rel="canonical" href="https://www.vola.ai/integrations/claude" />`,
		`<meta property="og:url" content="https://www.vola.ai/integrations/claude" />`,
		`"@type":"SoftwareApplication"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered index missing %q:\n%s", want, rendered)
		}
	}
}
