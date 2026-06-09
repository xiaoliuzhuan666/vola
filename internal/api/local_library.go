package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
)

const (
	localLibraryVersion     = "vola.local_library_scan/v1"
	localLibraryProjectName = "local-knowledge-index"
	maxMarkdownReadBytes    = 64 * 1024
)

type localLibraryScanRequest struct {
	Roots       []string `json:"roots,omitempty"`
	MaxMarkdown int      `json:"max_markdown,omitempty"`
	MaxProjects int      `json:"max_projects,omitempty"`
}

type localLibraryImportRequest struct {
	Roots       []string `json:"roots,omitempty"`
	MaxMarkdown int      `json:"max_markdown,omitempty"`
	MaxProjects int      `json:"max_projects,omitempty"`
}

type localLibraryRoot struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Scanned bool   `json:"scanned"`
	Error   string `json:"error,omitempty"`
}

type localLibraryStats struct {
	RootsRequested int `json:"roots_requested"`
	RootsScanned   int `json:"roots_scanned"`
	MarkdownFound  int `json:"markdown_found"`
	MarkdownShown  int `json:"markdown_shown"`
	ProjectsFound  int `json:"projects_found"`
	ProjectsShown  int `json:"projects_shown"`
	DirsSkipped    int `json:"dirs_skipped"`
	FilesScanned   int `json:"files_scanned"`
	SensitiveFiles int `json:"sensitive_files"`
}

type localLibraryProjectCandidate struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Score         int      `json:"score"`
	Markers       []string `json:"markers"`
	Reasons       []string `json:"reasons"`
	MarkdownCount int      `json:"markdown_count"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

type localLibraryMarkdownCandidate struct {
	Title              string   `json:"title"`
	Path               string   `json:"path"`
	ProjectName        string   `json:"project_name,omitempty"`
	ProjectPath        string   `json:"project_path,omitempty"`
	Category           string   `json:"category"`
	GenericCandidate   bool     `json:"generic_candidate"`
	SensitiveCandidate bool     `json:"sensitive_candidate"`
	SizeBytes          int64    `json:"size_bytes"`
	UpdatedAt          string   `json:"updated_at,omitempty"`
	Score              int      `json:"score"`
	Headings           []string `json:"headings,omitempty"`
	Excerpt            string   `json:"excerpt,omitempty"`
}

type localLibraryScanResponse struct {
	Version     string                          `json:"version"`
	GeneratedAt string                          `json:"generated_at"`
	Roots       []localLibraryRoot              `json:"roots"`
	Stats       localLibraryStats               `json:"stats"`
	Projects    []localLibraryProjectCandidate  `json:"projects"`
	Markdown    []localLibraryMarkdownCandidate `json:"markdown"`
	Warnings    []string                        `json:"warnings,omitempty"`
}

type localLibraryImportResponse struct {
	Version     string            `json:"version"`
	GeneratedAt string            `json:"generated_at"`
	ProjectName string            `json:"project_name"`
	Paths       []string          `json:"paths"`
	Stats       localLibraryStats `json:"stats"`
	Warnings    []string          `json:"warnings,omitempty"`
}

type localLibraryScanner struct {
	maxMarkdown int
	maxProjects int
}

func (s *Server) handleLocalLibraryScan(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local library scan is only available in local mode")
		return
	}
	var req localLibraryScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	resp, err := runLocalLibraryScan(r.Context(), req.Roots, req.MaxMarkdown, req.MaxProjects)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, resp)
}

func (s *Server) handleLocalLibraryImport(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local library import is only available in local mode")
		return
	}
	if s.ProjectService == nil || s.FileTreeService == nil {
		respondNotConfigured(w, "project and file tree services")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req localLibraryImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	scan, err := runLocalLibraryScan(r.Context(), req.Roots, req.MaxMarkdown, req.MaxProjects)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	ctx := s.requestSourceContext(r, "local-library-scan")
	if _, err := s.ProjectService.Get(ctx, userID, localLibraryProjectName); err != nil {
		if _, err := s.ProjectService.Create(ctx, userID, localLibraryProjectName); err != nil {
			respondInternalError(w, err)
			return
		}
	}

	contextMD := renderLocalLibraryContext(scan)
	if err := s.ProjectService.UpdateContext(ctx, userID, localLibraryProjectName, contextMD); err != nil {
		respondInternalError(w, err)
		return
	}

	paths := []string{hubpath.ProjectContextPath(localLibraryProjectName)}
	writes := map[string]string{
		"project-index.md":  renderLocalLibraryProjectsMarkdown(scan),
		"markdown-index.md": renderLocalLibraryMarkdown(scan),
	}
	for filename, content := range writes {
		target := filepath.ToSlash(filepath.Join(hubpath.ProjectDir(localLibraryProjectName), filename))
		if _, err := s.FileTreeService.WriteEntry(ctx, userID, target, content, "text/markdown", models.FileTreeWriteOptions{
			Kind:          "project_file",
			MinTrustLevel: models.TrustLevelWork,
			Metadata: map[string]interface{}{
				"source":         services.SourceOrDefault(ctx, "local-library-scan"),
				"source_kind":    "local_library_scan",
				"generated_at":   scan.GeneratedAt,
				"source_roots":   localLibraryScannedRootPaths(scan.Roots),
				"source_project": localLibraryProjectName,
			},
		}); err != nil {
			respondInternalError(w, err)
			return
		}
		paths = append(paths, target)
	}

	indexJSON, err := json.MarshalIndent(scan, "", "  ")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	jsonPath := filepath.ToSlash(filepath.Join(hubpath.ProjectDir(localLibraryProjectName), "index.json"))
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, jsonPath, string(append(indexJSON, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind:          "project_file",
		MinTrustLevel: models.TrustLevelWork,
		Metadata: map[string]interface{}{
			"source":         services.SourceOrDefault(ctx, "local-library-scan"),
			"source_kind":    "local_library_scan",
			"generated_at":   scan.GeneratedAt,
			"source_roots":   localLibraryScannedRootPaths(scan.Roots),
			"source_project": localLibraryProjectName,
		},
	}); err != nil {
		respondInternalError(w, err)
		return
	}
	paths = append(paths, jsonPath)

	resp := localLibraryImportResponse{
		Version:     localLibraryVersion,
		GeneratedAt: scan.GeneratedAt,
		ProjectName: localLibraryProjectName,
		Paths:       paths,
		Stats:       scan.Stats,
		Warnings:    scan.Warnings,
	}
	respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), userID))
}

func runLocalLibraryScan(ctx context.Context, roots []string, maxMarkdown, maxProjects int) (*localLibraryScanResponse, error) {
	scanner := localLibraryScanner{
		maxMarkdown: maxMarkdown,
		maxProjects: maxProjects,
	}
	return scanner.scan(ctx, roots)
}

func (s localLibraryScanner) scan(ctx context.Context, roots []string) (*localLibraryScanResponse, error) {
	if s.maxMarkdown <= 0 {
		s.maxMarkdown = 5000
	}
	if s.maxProjects <= 0 {
		s.maxProjects = 500
	}
	resolvedRoots, err := resolveLocalLibraryRoots(roots)
	if err != nil {
		return nil, err
	}
	response := &localLibraryScanResponse{
		Version:     localLibraryVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Roots:       make([]localLibraryRoot, 0, len(resolvedRoots)),
		Projects:    []localLibraryProjectCandidate{},
		Markdown:    []localLibraryMarkdownCandidate{},
	}
	response.Stats.RootsRequested = len(resolvedRoots)

	projectMap := map[string]*localLibraryProjectCandidate{}
	var markdown []localLibraryMarkdownCandidate

	for _, root := range resolvedRoots {
		rootStatus := localLibraryRoot{Path: root}
		info, err := os.Stat(root)
		if err != nil {
			rootStatus.Exists = false
			rootStatus.Error = err.Error()
			response.Roots = append(response.Roots, rootStatus)
			continue
		}
		if !info.IsDir() {
			rootStatus.Exists = true
			rootStatus.Error = "not a directory"
			response.Roots = append(response.Roots, rootStatus)
			continue
		}
		rootStatus.Exists = true
		rootStatus.Scanned = true
		response.Roots = append(response.Roots, rootStatus)
		response.Stats.RootsScanned++

		err = filepath.WalkDir(root, func(pathValue string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				response.Warnings = append(response.Warnings, fmt.Sprintf("%s: %v", pathValue, walkErr))
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() {
				if pathValue != root && shouldSkipLocalLibraryDir(pathValue, d.Name()) {
					response.Stats.DirsSkipped++
					return filepath.SkipDir
				}
				if candidate := inspectLocalProjectCandidate(pathValue); candidate != nil {
					existing := projectMap[candidate.Path]
					if existing == nil || candidate.Score > existing.Score {
						projectMap[candidate.Path] = candidate
					}
				}
				return nil
			}
			response.Stats.FilesScanned++
			if !isMarkdownFilename(d.Name()) {
				return nil
			}
			candidate, err := inspectLocalMarkdown(pathValue)
			if err != nil {
				response.Warnings = append(response.Warnings, fmt.Sprintf("%s: %v", pathValue, err))
				return nil
			}
			if candidate.SensitiveCandidate {
				response.Stats.SensitiveFiles++
			}
			markdown = append(markdown, candidate)
			response.Stats.MarkdownFound++
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	projects := make([]localLibraryProjectCandidate, 0, len(projectMap))
	for _, project := range projectMap {
		if project.Score < 4 {
			continue
		}
		projects = append(projects, *project)
	}
	sortLocalProjects(projects)
	linkMarkdownToProjects(markdown, projects)
	sortLocalMarkdown(markdown)

	projectCounts := map[string]int{}
	for _, doc := range markdown {
		if doc.ProjectPath != "" {
			projectCounts[doc.ProjectPath]++
		}
	}
	for i := range projects {
		projects[i].MarkdownCount = projectCounts[projects[i].Path]
	}
	sortLocalProjects(projects)

	response.Stats.ProjectsFound = len(projects)
	if len(projects) > s.maxProjects {
		response.Projects = append(response.Projects, projects[:s.maxProjects]...)
	} else {
		response.Projects = append(response.Projects, projects...)
	}
	response.Stats.ProjectsShown = len(response.Projects)
	if len(markdown) > s.maxMarkdown {
		response.Markdown = append(response.Markdown, markdown[:s.maxMarkdown]...)
	} else {
		response.Markdown = append(response.Markdown, markdown...)
	}
	response.Stats.MarkdownShown = len(response.Markdown)
	return response, nil
}

func resolveLocalLibraryRoots(roots []string) ([]string, error) {
	if len(roots) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		roots = []string{
			filepath.Join(home, "Desktop"),
			filepath.Join(home, "Downloads"),
			filepath.Join(home, "Documents"),
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(roots))
	for _, raw := range roots {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			raw = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(raw, "~"), string(filepath.Separator)))
		}
		abs, err := filepath.Abs(raw)
		if err != nil {
			return nil, err
		}
		abs = filepath.Clean(abs)
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one root is required")
	}
	sort.Strings(out)
	return out, nil
}

func shouldSkipLocalLibraryDir(pathValue, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	skipNames := map[string]bool{
		".git": true, ".hg": true, ".svn": true, ".idea": true, ".vscode": true,
		"node_modules": true, "dist": true, "build": true, "target": true, "out": true,
		".next": true, ".nuxt": true, ".codegraph": true, ".venv": true, "venv": true,
		"__pycache__": true, "pods": true, "deriveddata": true, "coverage": true,
		"unpackage": true, ".playwright-mcp": true,
	}
	if skipNames[lower] {
		return true
	}
	clean := filepath.ToSlash(strings.ToLower(pathValue))
	return strings.Contains(clean, "/node_modules/") || strings.Contains(clean, "/.git/")
}

func inspectLocalProjectCandidate(dir string) *localLibraryProjectCandidate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	markers := []string{}
	reasons := []string{}
	score := 0
	hasGit := false
	hasDocs := false
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		switch lower {
		case ".git":
			if entry.IsDir() {
				hasGit = true
				score += 3
				reasons = append(reasons, "git repository")
			}
		case "docs":
			if entry.IsDir() {
				hasDocs = true
			}
		case "package.json", "go.mod", "cargo.toml", "pyproject.toml", "pom.xml", "build.gradle", "settings.gradle", "composer.json", "gemfile":
			markers = append(markers, name)
			score += 6
		case "dockerfile", "docker-compose.yml", "docker-compose.yaml":
			markers = append(markers, name)
			score += 4
		case "pages.json", "project.config.json", "vite.config.ts", "vite.config.js", "next.config.js", "next.config.mjs", "nuxt.config.ts":
			markers = append(markers, name)
			score += 4
		case "agents.md", "readme.md":
			markers = append(markers, name)
			score += 2
		}
	}
	if len(markers) == 0 && !hasGit {
		return nil
	}
	if hasDocs {
		score += 1
		reasons = append(reasons, "docs directory")
	}
	if isLikelyDependencyPath(dir) {
		score -= 5
		reasons = append(reasons, "dependency-like path")
	}
	if len(markers) > 0 {
		reasons = append(reasons, "project markers")
	}
	info, _ := os.Stat(dir)
	updatedAt := ""
	if info != nil {
		updatedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	return &localLibraryProjectCandidate{
		Name:      filepath.Base(dir),
		Path:      dir,
		Score:     score,
		Markers:   compactLocalLibraryStrings(markers, 12),
		Reasons:   compactLocalLibraryStrings(reasons, 6),
		UpdatedAt: updatedAt,
	}
}

func inspectLocalMarkdown(pathValue string) (localLibraryMarkdownCandidate, error) {
	info, err := os.Stat(pathValue)
	if err != nil {
		return localLibraryMarkdownCandidate{}, err
	}
	sensitive := isSensitiveLocalMarkdownPath(pathValue)
	doc := localLibraryMarkdownCandidate{
		Title:              markdownTitleFromFilename(pathValue),
		Path:               pathValue,
		Category:           "project-note",
		SensitiveCandidate: sensitive,
		SizeBytes:          info.Size(),
		UpdatedAt:          info.ModTime().UTC().Format(time.RFC3339),
	}
	if sensitive {
		doc.Category = "sensitive-metadata"
		doc.Score = scoreLocalMarkdown(doc)
		return doc, nil
	}
	data, err := readFilePrefix(pathValue, maxMarkdownReadBytes)
	if err != nil {
		return localLibraryMarkdownCandidate{}, err
	}
	text := string(data)
	title, headings := extractMarkdownTitleAndHeadings(text)
	if title != "" {
		doc.Title = title
	}
	doc.Headings = headings
	doc.Excerpt = localLibrarySnippet(firstMarkdownParagraphOrText(text))
	doc.Category, doc.GenericCandidate = classifyLocalMarkdown(pathValue, doc.Title, text)
	doc.Score = scoreLocalMarkdown(doc)
	return doc, nil
}

func readFilePrefix(pathValue string, limit int64) ([]byte, error) {
	file, err := os.Open(pathValue)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit))
}

func isMarkdownFilename(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}

func isLikelyDependencyPath(pathValue string) bool {
	clean := filepath.ToSlash(strings.ToLower(pathValue))
	return strings.Contains(clean, "/node_modules/") ||
		strings.Contains(clean, "/uni_modules/") ||
		strings.Contains(clean, "/vendor/") ||
		strings.Contains(clean, "/pods/")
}

func isSensitiveLocalMarkdownPath(pathValue string) bool {
	lower := strings.ToLower(filepath.ToSlash(pathValue))
	keywords := []string{"secret", "token", "credential", "password", "passwd", "api-key", "apikey", "recovery", "账号", "密码", "密钥", "授权", "凭证"}
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func extractMarkdownTitleAndHeadings(text string) (string, []string) {
	headings := []string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if heading == "" {
			continue
		}
		headings = append(headings, heading)
		if len(headings) >= 6 {
			break
		}
	}
	title := ""
	if len(headings) > 0 {
		title = headings[0]
	}
	return title, headings
}

func markdownTitleFromFilename(pathValue string) string {
	name := filepath.Base(pathValue)
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return strings.TrimSpace(name)
}

func firstMarkdownParagraphOrText(text string) string {
	lines := strings.Split(text, "\n")
	parts := []string{}
	started := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if started {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "---") {
			if started {
				break
			}
			continue
		}
		started = true
		parts = append(parts, line)
		if len(strings.Join(parts, " ")) > 260 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func classifyLocalMarkdown(pathValue, title, text string) (string, bool) {
	haystack := strings.ToLower(filepath.ToSlash(pathValue) + " " + title + " " + firstNRunes(text, 2000))
	genericTerms := []string{
		"方案", "指南", "规范", "调研", "验收", "部署", "迁移", "备份", "恢复", "排查", "清单",
		"guide", "playbook", "runbook", "checklist", "best practice", "research", "deployment", "deploy", "docker", "backup", "restore",
	}
	for _, term := range genericTerms {
		if strings.Contains(haystack, strings.ToLower(term)) {
			return "general-playbook", true
		}
	}
	if strings.Contains(haystack, "/course") || strings.Contains(haystack, "课程") || strings.Contains(haystack, "学习") {
		return "learning-note", false
	}
	return "project-note", false
}

func scoreLocalMarkdown(doc localLibraryMarkdownCandidate) int {
	score := 0
	if doc.GenericCandidate {
		score += 20
	}
	if doc.ProjectPath != "" {
		score += 5
	}
	if len(doc.Headings) > 0 {
		score += 3
	}
	if doc.Excerpt != "" {
		score += 2
	}
	if doc.SensitiveCandidate {
		score -= 10
	}
	return score
}

func linkMarkdownToProjects(markdown []localLibraryMarkdownCandidate, projects []localLibraryProjectCandidate) {
	for i := range markdown {
		bestPath := ""
		bestName := ""
		for _, project := range projects {
			if !pathIsWithin(markdown[i].Path, project.Path) {
				continue
			}
			if len(project.Path) > len(bestPath) {
				bestPath = project.Path
				bestName = project.Name
			}
		}
		markdown[i].ProjectPath = bestPath
		markdown[i].ProjectName = bestName
		markdown[i].Score = scoreLocalMarkdown(markdown[i])
	}
}

func pathIsWithin(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "..")
}

func sortLocalProjects(projects []localLibraryProjectCandidate) {
	sort.SliceStable(projects, func(i, j int) bool {
		if projects[i].Score != projects[j].Score {
			return projects[i].Score > projects[j].Score
		}
		if projects[i].MarkdownCount != projects[j].MarkdownCount {
			return projects[i].MarkdownCount > projects[j].MarkdownCount
		}
		return projects[i].Path < projects[j].Path
	})
}

func sortLocalMarkdown(markdown []localLibraryMarkdownCandidate) {
	sort.SliceStable(markdown, func(i, j int) bool {
		if markdown[i].Score != markdown[j].Score {
			return markdown[i].Score > markdown[j].Score
		}
		if markdown[i].UpdatedAt != markdown[j].UpdatedAt {
			return markdown[i].UpdatedAt > markdown[j].UpdatedAt
		}
		return markdown[i].Path < markdown[j].Path
	})
}

func compactLocalLibraryStrings(values []string, limit int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func localLibrarySnippet(raw string) string {
	raw = strings.Join(strings.Fields(raw), " ")
	if len(raw) <= 240 {
		return raw
	}
	truncated := raw[:237]
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return strings.TrimSpace(truncated) + "..."
}

func firstNRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	count := 0
	for idx := range text {
		if count >= limit {
			return text[:idx]
		}
		count++
	}
	return text
}

func renderLocalLibraryContext(scan *localLibraryScanResponse) string {
	var b strings.Builder
	b.WriteString("# Local Knowledge Index\n\n")
	b.WriteString("本地资料索引：记录桌面、下载、文稿中的项目候选和 Markdown 路径，原文件不会被移动或改写。\n\n")
	b.WriteString(fmt.Sprintf("- Generated at: %s\n", scan.GeneratedAt))
	b.WriteString(fmt.Sprintf("- Roots scanned: %d / %d\n", scan.Stats.RootsScanned, scan.Stats.RootsRequested))
	b.WriteString(fmt.Sprintf("- Project candidates: %d\n", scan.Stats.ProjectsFound))
	b.WriteString(fmt.Sprintf("- Markdown files: %d\n", scan.Stats.MarkdownFound))
	b.WriteString(fmt.Sprintf("- Sensitive-looking Markdown files: %d\n\n", scan.Stats.SensitiveFiles))
	b.WriteString("## Scanned Roots\n\n")
	for _, root := range scan.Roots {
		status := "scanned"
		if !root.Exists {
			status = "missing"
		} else if !root.Scanned {
			status = "skipped"
		}
		b.WriteString(fmt.Sprintf("- `%s` - %s", root.Path, status))
		if root.Error != "" {
			b.WriteString(fmt.Sprintf(" (%s)", root.Error))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Fast Search Hints\n\n")
	b.WriteString("- Search this project for a customer name, product name, API name, cloud provider, deployment tool, or error keyword.\n")
	b.WriteString("- Open `project-index.md` when you need to find a repository or app folder.\n")
	b.WriteString("- Open `markdown-index.md` when you need a reusable plan, checklist, deployment note, or research record.\n\n")
	b.WriteString("## Top Project Candidates\n\n")
	b.WriteString("| Name | Score | Markdown | Path |\n| --- | ---: | ---: | --- |\n")
	for _, project := range firstLocalProjects(scan.Projects, 25) {
		b.WriteString(fmt.Sprintf("| %s | %d | %d | `%s` |\n", markdownTableCell(project.Name), project.Score, project.MarkdownCount, project.Path))
	}
	b.WriteString("\n## Top Markdown Candidates\n\n")
	b.WriteString("| Title | Category | Project | Path |\n| --- | --- | --- | --- |\n")
	for _, doc := range firstLocalMarkdown(scan.Markdown, 35) {
		project := doc.ProjectName
		if project == "" {
			project = "-"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | `%s` |\n", markdownTableCell(doc.Title), markdownTableCell(doc.Category), markdownTableCell(project), doc.Path))
	}
	return b.String()
}

func renderLocalLibraryProjectsMarkdown(scan *localLibraryScanResponse) string {
	var b strings.Builder
	b.WriteString("# Local Project Candidates\n\n")
	b.WriteString(fmt.Sprintf("Generated at: %s\n\n", scan.GeneratedAt))
	b.WriteString("| Name | Score | Markdown | Markers | Path |\n| --- | ---: | ---: | --- | --- |\n")
	for _, project := range scan.Projects {
		b.WriteString(fmt.Sprintf(
			"| %s | %d | %d | %s | `%s` |\n",
			markdownTableCell(project.Name),
			project.Score,
			project.MarkdownCount,
			markdownTableCell(strings.Join(project.Markers, ", ")),
			project.Path,
		))
	}
	return b.String()
}

func renderLocalLibraryMarkdown(scan *localLibraryScanResponse) string {
	var b strings.Builder
	b.WriteString("# Local Markdown Index\n\n")
	b.WriteString(fmt.Sprintf("Generated at: %s\n\n", scan.GeneratedAt))
	b.WriteString("| Title | Category | Project | Sensitive | Path |\n| --- | --- | --- | --- | --- |\n")
	for _, doc := range scan.Markdown {
		project := doc.ProjectName
		if project == "" {
			project = "-"
		}
		sensitive := "no"
		if doc.SensitiveCandidate {
			sensitive = "yes"
		}
		b.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %s | `%s` |\n",
			markdownTableCell(doc.Title),
			markdownTableCell(doc.Category),
			markdownTableCell(project),
			sensitive,
			doc.Path,
		))
		if doc.Excerpt != "" {
			b.WriteString(fmt.Sprintf("<!-- %s -->\n", strings.ReplaceAll(doc.Excerpt, "--", "- -")))
		}
	}
	return b.String()
}

func firstLocalProjects(values []localLibraryProjectCandidate, limit int) []localLibraryProjectCandidate {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstLocalMarkdown(values []localLibraryMarkdownCandidate, limit int) []localLibraryMarkdownCandidate {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}

func localLibraryScannedRootPaths(roots []localLibraryRoot) []string {
	out := []string{}
	for _, root := range roots {
		if root.Scanned {
			out = append(out, root.Path)
		}
	}
	return out
}
