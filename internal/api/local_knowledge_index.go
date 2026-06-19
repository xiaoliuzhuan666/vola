package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	localKnowledgeIndexVersion = "vola.local_knowledge_index/v1"
	maxKnowledgeIndexReadBytes = 256 * 1024
)

type localKnowledgeIndexRequest struct {
	Roots       []string `json:"roots,omitempty"`
	MaxMarkdown int      `json:"max_markdown,omitempty"`
	MaxProjects int      `json:"max_projects,omitempty"`
}

type localKnowledgeHeading struct {
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
}

type localKnowledgeLink struct {
	SourcePath string `json:"source_path,omitempty"`
	Kind       string `json:"kind"`
	Text       string `json:"text"`
	Target     string `json:"target"`
	TargetPath string `json:"target_path,omitempty"`
	Resolved   bool   `json:"resolved"`
}

type localKnowledgeBacklink struct {
	SourcePath  string `json:"source_path"`
	SourceTitle string `json:"source_title"`
	Kind        string `json:"kind"`
	Text        string `json:"text"`
}

type localKnowledgeDocument struct {
	ID                  string                   `json:"id"`
	Title               string                   `json:"title"`
	Path                string                   `json:"path"`
	ProjectName         string                   `json:"project_name,omitempty"`
	ProjectPath         string                   `json:"project_path,omitempty"`
	Category            string                   `json:"category"`
	GenericCandidate    bool                     `json:"generic_candidate"`
	SensitiveCandidate  bool                     `json:"sensitive_candidate"`
	SizeBytes           int64                    `json:"size_bytes"`
	UpdatedAt           string                   `json:"updated_at,omitempty"`
	Score               int                      `json:"score"`
	Headings            []string                 `json:"headings,omitempty"`
	HeadingItems        []localKnowledgeHeading  `json:"heading_items,omitempty"`
	Concepts            []string                 `json:"concepts,omitempty"`
	OutgoingLinks       []localKnowledgeLink     `json:"outgoing_links,omitempty"`
	Backlinks           []localKnowledgeBacklink `json:"backlinks,omitempty"`
	Excerpt             string                   `json:"excerpt,omitempty"`
	Summary             string                   `json:"summary,omitempty"`
	SuggestedOutputPath string                   `json:"suggested_output_path,omitempty"`
}

type localKnowledgeConcept struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Category      string   `json:"category"`
	Count         int      `json:"count"`
	DocumentPaths []string `json:"document_paths"`
	Related       []string `json:"related,omitempty"`
}

type localKnowledgeTreeNode struct {
	ID       string                   `json:"id"`
	Kind     string                   `json:"kind"`
	Label    string                   `json:"label"`
	Path     string                   `json:"path,omitempty"`
	Count    int                      `json:"count,omitempty"`
	Children []localKnowledgeTreeNode `json:"children,omitempty"`
}

type localKnowledgeCompilePlan struct {
	OutputDir   string   `json:"output_dir"`
	SourceDirs  []string `json:"source_dirs"`
	Steps       []string `json:"steps"`
	Prompt      string   `json:"prompt"`
	SourcePaths []string `json:"source_paths"`
}

type localKnowledgeIndexResponse struct {
	Version     string                         `json:"version"`
	GeneratedAt string                         `json:"generated_at"`
	Roots       []localLibraryRoot             `json:"roots"`
	Stats       localLibraryStats              `json:"stats"`
	Projects    []localLibraryProjectCandidate `json:"projects"`
	Documents   []localKnowledgeDocument       `json:"documents"`
	Concepts    []localKnowledgeConcept        `json:"concepts"`
	Links       []localKnowledgeLink           `json:"links"`
	Tree        []localKnowledgeTreeNode       `json:"tree"`
	Compile     localKnowledgeCompilePlan      `json:"compile"`
	Warnings    []string                       `json:"warnings,omitempty"`
}

type parsedKnowledgeDocument struct {
	doc   localKnowledgeDocument
	links []localKnowledgeLink
}

func (s *Server) handleLocalKnowledgeIndex(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local knowledge index is only available in local mode")
		return
	}
	var req localKnowledgeIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	resp, err := buildLocalKnowledgeIndex(r.Context(), req.Roots, req.MaxMarkdown, req.MaxProjects)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, resp)
}

func buildLocalKnowledgeIndex(ctx context.Context, roots []string, maxMarkdown, maxProjects int) (*localKnowledgeIndexResponse, error) {
	scan, err := runLocalLibraryScan(ctx, roots, maxMarkdown, maxProjects)
	if err != nil {
		return nil, err
	}
	index := &localKnowledgeIndexResponse{
		Version:     localKnowledgeIndexVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Roots:       scan.Roots,
		Stats:       scan.Stats,
		Projects:    scan.Projects,
		Documents:   []localKnowledgeDocument{},
		Concepts:    []localKnowledgeConcept{},
		Links:       []localKnowledgeLink{},
		Tree:        []localKnowledgeTreeNode{},
		Warnings:    append([]string{}, scan.Warnings...),
	}

	parsed := make([]parsedKnowledgeDocument, 0, len(scan.Markdown))
	for _, candidate := range scan.Markdown {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if candidate.SensitiveCandidate {
			continue
		}
		doc, links, err := parseKnowledgeDocument(candidate)
		if err != nil {
			index.Warnings = append(index.Warnings, fmt.Sprintf("%s: %v", candidate.Path, err))
			continue
		}
		parsed = append(parsed, parsedKnowledgeDocument{doc: doc, links: links})
	}

	resolveKnowledgeLinks(parsed)
	index.Documents = make([]localKnowledgeDocument, 0, len(parsed))
	for _, item := range parsed {
		index.Documents = append(index.Documents, item.doc)
		index.Links = append(index.Links, item.doc.OutgoingLinks...)
	}
	sort.SliceStable(index.Links, func(i, j int) bool {
		if index.Links[i].SourcePath != index.Links[j].SourcePath {
			return index.Links[i].SourcePath < index.Links[j].SourcePath
		}
		return index.Links[i].Target < index.Links[j].Target
	})
	index.Concepts = buildKnowledgeConcepts(index.Documents)
	index.Tree = buildKnowledgeTree(index.Projects, index.Documents, index.Concepts)
	index.Compile = buildKnowledgeCompilePlan(index)
	return index, nil
}

func parseKnowledgeDocument(candidate localLibraryMarkdownCandidate) (localKnowledgeDocument, []localKnowledgeLink, error) {
	data, err := readFilePrefix(candidate.Path, maxKnowledgeIndexReadBytes)
	if err != nil {
		return localKnowledgeDocument{}, nil, err
	}
	text := string(data)
	headingItems, headingTexts := extractKnowledgeHeadings(text)
	links := extractKnowledgeLinks(candidate.Path, text)
	concepts := deriveKnowledgeConcepts(candidate, headingItems, links, text)
	summary := localLibrarySnippet(firstMarkdownParagraphOrText(text))
	if summary == "" {
		summary = candidate.Excerpt
	}
	doc := localKnowledgeDocument{
		ID:                  knowledgeID(candidate.Path),
		Title:               candidate.Title,
		Path:                candidate.Path,
		ProjectName:         candidate.ProjectName,
		ProjectPath:         candidate.ProjectPath,
		Category:            candidate.Category,
		GenericCandidate:    candidate.GenericCandidate,
		SensitiveCandidate:  candidate.SensitiveCandidate,
		SizeBytes:           candidate.SizeBytes,
		UpdatedAt:           candidate.UpdatedAt,
		Score:               candidate.Score,
		Headings:            headingTexts,
		HeadingItems:        headingItems,
		Concepts:            concepts,
		OutgoingLinks:       links,
		Backlinks:           []localKnowledgeBacklink{},
		Excerpt:             candidate.Excerpt,
		Summary:             summary,
		SuggestedOutputPath: suggestedKnowledgeOutputPath(candidate),
	}
	if len(headingItems) > 0 && strings.TrimSpace(headingItems[0].Text) != "" {
		doc.Title = headingItems[0].Text
	}
	return doc, links, nil
}

func extractKnowledgeHeadings(text string) ([]localKnowledgeHeading, []string) {
	items := []localKnowledgeHeading{}
	texts := []string{}
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level == 0 || level > 6 {
			continue
		}
		if level < len(trimmed) && trimmed[level] != ' ' && trimmed[level] != '\t' {
			continue
		}
		textValue := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		textValue = strings.Trim(textValue, "# \t")
		if textValue == "" {
			continue
		}
		items = append(items, localKnowledgeHeading{Level: level, Text: textValue, Anchor: knowledgeAnchor(textValue)})
		texts = append(texts, textValue)
		if len(items) >= 80 {
			break
		}
	}
	return items, texts
}

var (
	wikiLinkRe     = regexp.MustCompile(`\[\[([^\]\|#]+)(?:#[^\]\|]+)?(?:\|([^\]]+))?\]\]`)
	markdownLinkRe = regexp.MustCompile(`!?\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	frontmatterTag = regexp.MustCompile(`(?m)^\s*tags\s*:\s*(.+)$`)
	inlineTagRe    = regexp.MustCompile(`(^|\s)#([\p{Han}A-Za-z][\p{Han}A-Za-z0-9_-]{1,40})`)
)

func extractKnowledgeLinks(sourcePath, text string) []localKnowledgeLink {
	links := []localKnowledgeLink{}
	for _, match := range wikiLinkRe.FindAllStringSubmatch(text, -1) {
		target := strings.TrimSpace(match[1])
		label := target
		if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
			label = strings.TrimSpace(match[2])
		}
		if target == "" {
			continue
		}
		links = append(links, localKnowledgeLink{
			SourcePath: sourcePath,
			Kind:       "wiki",
			Text:       label,
			Target:     target,
		})
		if len(links) >= 80 {
			return compactKnowledgeLinks(links)
		}
	}
	for _, match := range markdownLinkRe.FindAllStringSubmatch(text, -1) {
		label := strings.TrimSpace(match[1])
		target := strings.TrimSpace(match[2])
		if target == "" || strings.HasPrefix(target, "#") {
			continue
		}
		kind := "markdown"
		if strings.HasPrefix(strings.ToLower(target), "http://") || strings.HasPrefix(strings.ToLower(target), "https://") {
			kind = "external"
		}
		if label == "" {
			label = target
		}
		links = append(links, localKnowledgeLink{
			SourcePath: sourcePath,
			Kind:       kind,
			Text:       label,
			Target:     target,
		})
		if len(links) >= 80 {
			return compactKnowledgeLinks(links)
		}
	}
	return compactKnowledgeLinks(links)
}

func compactKnowledgeLinks(links []localKnowledgeLink) []localKnowledgeLink {
	seen := map[string]bool{}
	out := []localKnowledgeLink{}
	for _, link := range links {
		key := link.Kind + "\x00" + link.Target + "\x00" + link.Text
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, link)
		if len(out) >= 60 {
			break
		}
	}
	return out
}

func resolveKnowledgeLinks(parsed []parsedKnowledgeDocument) {
	pathToIndex := map[string]int{}
	titleToPath := map[string]string{}
	baseToPath := map[string]string{}
	for i := range parsed {
		pathToIndex[parsed[i].doc.Path] = i
		titleSlug := knowledgeSlug(parsed[i].doc.Title)
		if titleSlug != "" && titleToPath[titleSlug] == "" {
			titleToPath[titleSlug] = parsed[i].doc.Path
		}
		base := strings.TrimSuffix(filepath.Base(parsed[i].doc.Path), filepath.Ext(parsed[i].doc.Path))
		baseSlug := knowledgeSlug(base)
		if baseSlug != "" && baseToPath[baseSlug] == "" {
			baseToPath[baseSlug] = parsed[i].doc.Path
		}
	}

	for i := range parsed {
		sourcePath := parsed[i].doc.Path
		for linkIndex := range parsed[i].doc.OutgoingLinks {
			link := &parsed[i].doc.OutgoingLinks[linkIndex]
			targetPath := resolveKnowledgeTarget(sourcePath, *link, pathToIndex, titleToPath, baseToPath)
			if targetPath == "" {
				continue
			}
			link.TargetPath = targetPath
			link.Resolved = true
			targetIndex, ok := pathToIndex[targetPath]
			if !ok || targetPath == sourcePath {
				continue
			}
			parsed[targetIndex].doc.Backlinks = append(parsed[targetIndex].doc.Backlinks, localKnowledgeBacklink{
				SourcePath:  sourcePath,
				SourceTitle: parsed[i].doc.Title,
				Kind:        link.Kind,
				Text:        link.Text,
			})
		}
	}

	for i := range parsed {
		sort.SliceStable(parsed[i].doc.Backlinks, func(left, right int) bool {
			return parsed[i].doc.Backlinks[left].SourcePath < parsed[i].doc.Backlinks[right].SourcePath
		})
		if len(parsed[i].doc.Backlinks) > 40 {
			parsed[i].doc.Backlinks = parsed[i].doc.Backlinks[:40]
		}
	}
}

func resolveKnowledgeTarget(sourcePath string, link localKnowledgeLink, pathToIndex map[string]int, titleToPath, baseToPath map[string]string) string {
	target := stripKnowledgeTargetFragment(link.Target)
	if target == "" || link.Kind == "external" {
		return ""
	}
	if link.Kind == "wiki" {
		slug := knowledgeSlug(target)
		if path := titleToPath[slug]; path != "" {
			return path
		}
		if path := baseToPath[slug]; path != "" {
			return path
		}
		if path := baseToPath[knowledgeSlug(target+".md")]; path != "" {
			return path
		}
		return ""
	}

	candidates := []string{}
	if filepath.IsAbs(target) {
		candidates = append(candidates, filepath.Clean(target))
	} else {
		base := filepath.Dir(sourcePath)
		candidates = append(candidates, filepath.Clean(filepath.Join(base, target)))
		if filepath.Ext(target) == "" {
			candidates = append(candidates, filepath.Clean(filepath.Join(base, target+".md")))
			candidates = append(candidates, filepath.Clean(filepath.Join(base, target, "README.md")))
		}
	}
	for _, candidate := range candidates {
		if _, ok := pathToIndex[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func stripKnowledgeTargetFragment(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	for _, sep := range []string{"#", "?", "&"} {
		if idx := strings.Index(target, sep); idx >= 0 {
			target = target[:idx]
		}
	}
	return strings.TrimSpace(target)
}

func deriveKnowledgeConcepts(candidate localLibraryMarkdownCandidate, headings []localKnowledgeHeading, links []localKnowledgeLink, text string) []string {
	values := []string{}
	add := func(value string) {
		value = normalizeKnowledgeConcept(value)
		if value != "" {
			values = append(values, value)
		}
	}
	if candidate.ProjectName != "" {
		add(candidate.ProjectName)
	}
	add(localKnowledgeCategoryName(candidate.Category))
	for _, heading := range headings {
		if heading.Level <= 2 {
			add(heading.Text)
		}
	}
	for _, link := range links {
		if link.Kind == "wiki" {
			add(link.Target)
		}
	}
	for _, tag := range extractKnowledgeTags(text) {
		add(tag)
	}
	return compactLocalLibraryStrings(values, 16)
}

func extractKnowledgeTags(text string) []string {
	tags := []string{}
	if match := frontmatterTag.FindStringSubmatch(text); len(match) > 1 {
		raw := strings.TrimSpace(strings.Trim(match[1], "[]"))
		for _, part := range strings.Split(raw, ",") {
			tags = append(tags, strings.Trim(strings.TrimSpace(part), `"'`))
		}
	}
	for _, match := range inlineTagRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 {
			tags = append(tags, match[2])
		}
		if len(tags) >= 20 {
			break
		}
	}
	return compactLocalLibraryStrings(tags, 20)
}

func localKnowledgeCategoryName(category string) string {
	switch category {
	case "skill":
		return "Skill"
	case "agent-instructions":
		return "Agent Instructions"
	case "codex-note":
		return "Codex"
	case "general-playbook":
		return "Playbook"
	case "learning-note":
		return "Learning"
	case "project-note":
		return "Project Notes"
	default:
		return category
	}
}

func normalizeKnowledgeConcept(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "[]#`*_ \t\r\n")
	value = strings.ReplaceAll(value, "\\", "/")
	if strings.Contains(value, "/") {
		value = filepath.Base(value)
	}
	value = strings.TrimSuffix(value, filepath.Ext(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runeCount := utf8.RuneCountInString(value)
	if runeCount < 2 || runeCount > 48 {
		return ""
	}
	lower := strings.ToLower(value)
	stop := map[string]bool{
		"readme": true, "index": true, "overview": true, "untitled": true, "docs": true, "doc": true,
		"文档": true, "说明": true, "概览": true, "目录": true, "摘要": true,
	}
	if stop[lower] {
		return ""
	}
	return value
}

func buildKnowledgeConcepts(docs []localKnowledgeDocument) []localKnowledgeConcept {
	type conceptAccumulator struct {
		name      string
		category  string
		docs      map[string]bool
		related   map[string]int
		mentionCt int
	}
	acc := map[string]*conceptAccumulator{}
	for _, doc := range docs {
		docConcepts := compactLocalLibraryStrings(doc.Concepts, 16)
		for _, concept := range docConcepts {
			slug := knowledgeSlug(concept)
			if slug == "" {
				continue
			}
			if acc[slug] == nil {
				acc[slug] = &conceptAccumulator{
					name:     concept,
					category: conceptCategory(concept, doc.Category),
					docs:     map[string]bool{},
					related:  map[string]int{},
				}
			}
			acc[slug].docs[doc.Path] = true
			acc[slug].mentionCt++
			for _, related := range docConcepts {
				relatedSlug := knowledgeSlug(related)
				if relatedSlug == "" || relatedSlug == slug {
					continue
				}
				acc[slug].related[related]++
			}
		}
	}

	concepts := make([]localKnowledgeConcept, 0, len(acc))
	for slug, item := range acc {
		paths := make([]string, 0, len(item.docs))
		for path := range item.docs {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		relatedNames := make([]string, 0, len(item.related))
		for name := range item.related {
			relatedNames = append(relatedNames, name)
		}
		sort.SliceStable(relatedNames, func(i, j int) bool {
			if item.related[relatedNames[i]] != item.related[relatedNames[j]] {
				return item.related[relatedNames[i]] > item.related[relatedNames[j]]
			}
			return relatedNames[i] < relatedNames[j]
		})
		if len(relatedNames) > 8 {
			relatedNames = relatedNames[:8]
		}
		concepts = append(concepts, localKnowledgeConcept{
			Name:          item.name,
			Slug:          slug,
			Category:      item.category,
			Count:         len(paths),
			DocumentPaths: paths,
			Related:       relatedNames,
		})
	}
	sort.SliceStable(concepts, func(i, j int) bool {
		if concepts[i].Count != concepts[j].Count {
			return concepts[i].Count > concepts[j].Count
		}
		return concepts[i].Name < concepts[j].Name
	})
	if len(concepts) > 240 {
		concepts = concepts[:240]
	}
	return concepts
}

func conceptCategory(concept, fallback string) string {
	lower := strings.ToLower(concept)
	switch {
	case strings.Contains(lower, "skill"):
		return "skill"
	case strings.Contains(lower, "mcp"):
		return "mcp"
	case strings.Contains(lower, "codex") || strings.Contains(lower, "agent"):
		return "agent"
	case strings.Contains(lower, "deploy") || strings.Contains(lower, "release") || strings.Contains(concept, "部署") || strings.Contains(concept, "发布"):
		return "delivery"
	case strings.Contains(lower, "research") || strings.Contains(concept, "调研"):
		return "research"
	default:
		if fallback == "" {
			return "concept"
		}
		return fallback
	}
}

func buildKnowledgeTree(projects []localLibraryProjectCandidate, docs []localKnowledgeDocument, concepts []localKnowledgeConcept) []localKnowledgeTreeNode {
	docsByPath := map[string]localKnowledgeDocument{}
	for _, doc := range docs {
		docsByPath[doc.Path] = doc
	}

	projectGroups := map[string][]localKnowledgeDocument{}
	for _, doc := range docs {
		projectKey := doc.ProjectPath
		if projectKey == "" {
			projectKey = "unassigned"
		}
		projectGroups[projectKey] = append(projectGroups[projectKey], doc)
	}
	projectNameByPath := map[string]string{}
	for _, project := range projects {
		projectNameByPath[project.Path] = project.Name
	}
	projectNodes := make([]localKnowledgeTreeNode, 0, len(projectGroups))
	for projectPath, projectDocs := range projectGroups {
		sort.SliceStable(projectDocs, func(i, j int) bool {
			if projectDocs[i].Category != projectDocs[j].Category {
				return projectDocs[i].Category < projectDocs[j].Category
			}
			return projectDocs[i].Path < projectDocs[j].Path
		})
		label := projectNameByPath[projectPath]
		if projectPath == "unassigned" {
			label = "未归属项目"
		} else if label == "" {
			label = filepath.Base(projectPath)
		}
		projectNode := localKnowledgeTreeNode{
			ID:    "project:" + knowledgeID(projectPath),
			Kind:  "project",
			Label: label,
			Path:  projectPath,
			Count: len(projectDocs),
		}
		categoryDocs := map[string][]localKnowledgeDocument{}
		for _, doc := range projectDocs {
			categoryDocs[doc.Category] = append(categoryDocs[doc.Category], doc)
		}
		categories := make([]string, 0, len(categoryDocs))
		for category := range categoryDocs {
			categories = append(categories, category)
		}
		sort.Strings(categories)
		for _, category := range categories {
			children := []localKnowledgeTreeNode{}
			for _, doc := range firstKnowledgeDocs(categoryDocs[category], 80) {
				children = append(children, knowledgeDocTreeNode(doc))
			}
			projectNode.Children = append(projectNode.Children, localKnowledgeTreeNode{
				ID:       "category:" + knowledgeID(projectPath+":"+category),
				Kind:     "category",
				Label:    localKnowledgeCategoryName(category),
				Count:    len(categoryDocs[category]),
				Children: children,
			})
		}
		projectNodes = append(projectNodes, projectNode)
	}
	sort.SliceStable(projectNodes, func(i, j int) bool {
		if projectNodes[i].Count != projectNodes[j].Count {
			return projectNodes[i].Count > projectNodes[j].Count
		}
		return projectNodes[i].Label < projectNodes[j].Label
	})

	conceptNodes := []localKnowledgeTreeNode{}
	for _, concept := range firstKnowledgeConcepts(concepts, 80) {
		children := []localKnowledgeTreeNode{}
		for _, path := range firstKnowledgePaths(concept.DocumentPaths, 20) {
			if doc, ok := docsByPath[path]; ok {
				children = append(children, knowledgeDocTreeNode(doc))
			}
		}
		conceptNodes = append(conceptNodes, localKnowledgeTreeNode{
			ID:       "concept:" + concept.Slug,
			Kind:     "concept",
			Label:    concept.Name,
			Count:    concept.Count,
			Children: children,
		})
	}

	linkedDocs := []localKnowledgeDocument{}
	for _, doc := range docs {
		if len(doc.Backlinks) > 0 || len(doc.OutgoingLinks) > 0 {
			linkedDocs = append(linkedDocs, doc)
		}
	}
	sort.SliceStable(linkedDocs, func(i, j int) bool {
		left := len(linkedDocs[i].Backlinks) + len(linkedDocs[i].OutgoingLinks)
		right := len(linkedDocs[j].Backlinks) + len(linkedDocs[j].OutgoingLinks)
		if left != right {
			return left > right
		}
		return linkedDocs[i].Path < linkedDocs[j].Path
	})
	linkNodes := []localKnowledgeTreeNode{}
	for _, doc := range firstKnowledgeDocs(linkedDocs, 80) {
		linkNodes = append(linkNodes, knowledgeDocTreeNode(doc))
	}

	return []localKnowledgeTreeNode{
		{ID: "projects", Kind: "section", Label: "项目", Count: len(projectNodes), Children: firstKnowledgeNodes(projectNodes, 80)},
		{ID: "concepts", Kind: "section", Label: "概念", Count: len(concepts), Children: conceptNodes},
		{ID: "links", Kind: "section", Label: "链接关系", Count: len(linkNodes), Children: linkNodes},
	}
}

func buildKnowledgeCompilePlan(index *localKnowledgeIndexResponse) localKnowledgeCompilePlan {
	sourcePaths := []string{}
	for _, doc := range firstKnowledgeDocs(index.Documents, 120) {
		sourcePaths = append(sourcePaths, doc.Path)
	}
	sourceDirs := []string{}
	for _, root := range index.Roots {
		if root.Scanned {
			sourceDirs = append(sourceDirs, root.Path)
		}
	}
	steps := []string{
		"读取选中的原始 Markdown，不改写原文件。",
		"为每个资料文件生成 3-5 句摘要，保留事实来源路径。",
		"提取稳定概念，按 Skill、MCP、项目事实、研究资料、交付记录分类。",
		"为高频概念撰写独立 Markdown 概念页，并使用 [[概念名]] 互相链接。",
		"生成反向链接表和孤立文档清单，方便下一次整理。",
	}
	prompt := buildKnowledgeCompilePrompt(sourcePaths, index.Concepts)
	return localKnowledgeCompilePlan{
		OutputDir:   ".vola/index",
		SourceDirs:  sourceDirs,
		Steps:       steps,
		Prompt:      prompt,
		SourcePaths: sourcePaths,
	}
}

func buildKnowledgeCompilePrompt(sourcePaths []string, concepts []localKnowledgeConcept) string {
	var b strings.Builder
	b.WriteString("请把下面的本地 Markdown 资料整理成一个结构化知识索引。\n\n")
	b.WriteString("要求：\n")
	b.WriteString("- 不改写原始文件，只在 `.vola/index/` 下生成索引 Markdown。\n")
	b.WriteString("- 每个源文件生成 3-5 句资料摘要，摘要后标注真实路径。\n")
	b.WriteString("- 提取稳定概念，为高频概念写独立概念页。\n")
	b.WriteString("- 使用 Obsidian 风格 `[[概念名]]` 建立内容间链接。\n")
	b.WriteString("- 生成 `README.md`、`concepts.md`、`backlinks.md`、`orphans.md` 四个入口文件。\n")
	b.WriteString("- 有不确定处写进 `open-questions.md`，不要编造。\n\n")
	if len(concepts) > 0 {
		b.WriteString("候选概念：\n")
		for _, concept := range firstKnowledgeConcepts(concepts, 24) {
			b.WriteString(fmt.Sprintf("- %s (%d 个文件)\n", concept.Name, concept.Count))
		}
		b.WriteString("\n")
	}
	b.WriteString("源文件路径：\n")
	for _, path := range sourcePaths {
		b.WriteString("- ")
		b.WriteString(path)
		b.WriteString("\n")
	}
	return b.String()
}

func knowledgeDocTreeNode(doc localKnowledgeDocument) localKnowledgeTreeNode {
	return localKnowledgeTreeNode{
		ID:    "doc:" + doc.ID,
		Kind:  "document",
		Label: doc.Title,
		Path:  doc.Path,
		Count: len(doc.Backlinks) + len(doc.OutgoingLinks),
	}
}

func firstKnowledgeDocs(values []localKnowledgeDocument, limit int) []localKnowledgeDocument {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstKnowledgeConcepts(values []localKnowledgeConcept, limit int) []localKnowledgeConcept {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstKnowledgePaths(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstKnowledgeNodes(values []localKnowledgeTreeNode, limit int) []localKnowledgeTreeNode {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func suggestedKnowledgeOutputPath(candidate localLibraryMarkdownCandidate) string {
	project := candidate.ProjectName
	if project == "" {
		project = "local"
	}
	name := knowledgeSlug(candidate.Title)
	if name == "" {
		name = knowledgeSlug(filepath.Base(candidate.Path))
	}
	if name == "" {
		name = knowledgeID(candidate.Path)
	}
	return filepath.ToSlash(filepath.Join(".vola", "index", project, name+".md"))
}

func knowledgeAnchor(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return r
		case unicode.IsSpace(r), r == '-', r == '_':
			return '-'
		default:
			return -1
		}
	}, value)
	value = strings.Trim(value, "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}

func knowledgeSlug(value string) string {
	value = normalizeKnowledgeConcept(value)
	if value == "" {
		return ""
	}
	return knowledgeAnchor(value)
}

func knowledgeID(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}
