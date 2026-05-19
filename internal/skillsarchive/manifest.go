package skillsarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	ManifestVersion = "neudrive.skill-manifest/v1"
	ManifestFile    = "manifest.neudrive.json"
)

type SkillManifest struct {
	Version            string                   `json:"version"`
	SkillName          string                   `json:"skill_name"`
	EntryFile          string                   `json:"entry_file"`
	SourcePlatform     string                   `json:"source_platform,omitempty"`
	SourceArchive      string                   `json:"source_archive,omitempty"`
	Files              []SkillManifestFile      `json:"files"`
	ExternalReferences []SkillExternalReference `json:"external_references,omitempty"`
	EnvVars            []string                 `json:"env_vars,omitempty"`
	SupportedPlatforms []string                 `json:"supported_platforms,omitempty"`
	Warnings           []SkillManifestWarning   `json:"warnings,omitempty"`
	Summary            SkillManifestSummary     `json:"summary"`
}

type SkillManifestFile struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	SizeBytes   int    `json:"size_bytes"`
	ContentType string `json:"content_type,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Included    bool   `json:"included"`
}

type SkillExternalReference struct {
	Path       string `json:"path"`
	SourceFile string `json:"source_file"`
	Scope      string `json:"scope"`
	Included   bool   `json:"included"`
	Status     string `json:"status"`
}

type SkillManifestWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type SkillManifestSummary struct {
	Files              int `json:"files"`
	Scripts            int `json:"scripts"`
	DependencyFiles    int `json:"dependency_files"`
	Resources          int `json:"resources"`
	BinaryFiles        int `json:"binary_files"`
	LargeFiles         int `json:"large_files"`
	SecretRiskFiles    int `json:"secret_risk_files"`
	ExternalReferences int `json:"external_references"`
}

var (
	claudeExternalReferenceRE = regexp.MustCompile("(?:~|/)?/?\\.claude/(?:tools|plugins)/[^\\s\"'<>`)\\]]+")
	envAssignmentRE           = regexp.MustCompile(`(?m)(?:^|\s)(?:export\s+)?([A-Z][A-Z0-9_]{2,})\s*=`)
	envExpansionRE            = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]{2,})\}|\$([A-Z][A-Z0-9_]{2,})`)
)

func BuildManifests(entries []Entry, platform, archiveName string) []SkillManifest {
	grouped := map[string][]Entry{}
	for _, entry := range entries {
		if entry.Generated {
			continue
		}
		grouped[entry.SkillName] = append(grouped[entry.SkillName], entry)
	}
	skillNames := make([]string, 0, len(grouped))
	for skillName := range grouped {
		skillNames = append(skillNames, skillName)
	}
	sort.Strings(skillNames)

	manifests := make([]SkillManifest, 0, len(skillNames))
	for _, skillName := range skillNames {
		manifests = append(manifests, buildManifest(skillName, grouped[skillName], platform, archiveName))
	}
	return manifests
}

func AppendManifestEntries(entries []Entry, manifests []SkillManifest) ([]Entry, error) {
	if len(manifests) == 0 {
		return entries, nil
	}
	next := append([]Entry{}, entries...)
	for _, manifest := range manifests {
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		next = append(next, Entry{
			SkillName: manifest.SkillName,
			RelPath:   ManifestFile,
			Data:      data,
			Generated: true,
		})
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].SkillName == next[j].SkillName {
			return next[i].RelPath < next[j].RelPath
		}
		return next[i].SkillName < next[j].SkillName
	})
	return next, nil
}

func ManifestWarnings(manifests []SkillManifest) []SkillManifestWarning {
	warnings := []SkillManifestWarning{}
	for _, manifest := range manifests {
		warnings = append(warnings, manifest.Warnings...)
	}
	return warnings
}

func buildManifest(skillName string, entries []Entry, platform, archiveName string) SkillManifest {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelPath < entries[j].RelPath
	})

	manifest := SkillManifest{
		Version:        ManifestVersion,
		SkillName:      skillName,
		EntryFile:      "SKILL.md",
		SourcePlatform: strings.TrimSpace(platform),
		SourceArchive:  strings.TrimSpace(archiveName),
	}
	envVars := map[string]struct{}{}
	platforms := map[string]struct{}{}
	if normalized := strings.TrimSpace(platform); normalized != "" {
		platforms[normalized] = struct{}{}
	}
	includedExternalRefs := includedExternalReferenceKeys(entries)
	externalSeen := map[string]struct{}{}

	for _, entry := range entries {
		if strings.EqualFold(entry.RelPath, ManifestFile) {
			continue
		}
		contentType := DetectContentType(entry.RelPath, entry.Data)
		fileKind := classifyManifestFile(entry.RelPath, entry.Data)
		sum := sha256.Sum256(entry.Data)
		manifest.Files = append(manifest.Files, SkillManifestFile{
			Path:        entry.RelPath,
			Kind:        fileKind,
			SizeBytes:   len(entry.Data),
			ContentType: contentType,
			SHA256:      hex.EncodeToString(sum[:]),
			Included:    true,
		})
		manifest.Summary.Files++
		switch fileKind {
		case "script":
			manifest.Summary.Scripts++
		case "dependency":
			manifest.Summary.DependencyFiles++
		case "resource":
			manifest.Summary.Resources++
		case "binary":
			manifest.Summary.BinaryFiles++
		}
		if len(entry.Data) > 5<<20 {
			manifest.Summary.LargeFiles++
			manifest.Warnings = append(manifest.Warnings, SkillManifestWarning{
				Code:     "large_file",
				Severity: "warning",
				Path:     entry.RelPath,
				Message:  entry.RelPath + " is larger than 5 MB; verify that it should travel with the skill.",
			})
		}
		if looksSecretRiskPath(entry.RelPath) {
			manifest.Summary.SecretRiskFiles++
			manifest.Warnings = append(manifest.Warnings, SkillManifestWarning{
				Code:     "secret_risk",
				Severity: "warning",
				Path:     entry.RelPath,
				Message:  entry.RelPath + " looks like a secret-bearing file; verify before sharing or pushing to remote storage.",
			})
		}
		if LooksBinary(entry.RelPath, entry.Data) {
			manifest.Warnings = append(manifest.Warnings, SkillManifestWarning{
				Code:     "binary_asset",
				Severity: "info",
				Path:     entry.RelPath,
				Message:  entry.RelPath + " is stored as a binary skill asset.",
			})
			continue
		}

		content := string(entry.Data)
		for _, envName := range extractEnvVars(content) {
			envVars[envName] = struct{}{}
		}
		for _, platformName := range detectSupportedPlatforms(content) {
			platforms[platformName] = struct{}{}
		}
		for _, ref := range ExtractClaudeExternalReferences(content) {
			key := entry.RelPath + "\x00" + ref
			if _, ok := externalSeen[key]; ok {
				continue
			}
			externalSeen[key] = struct{}{}
			externalKey := claudeExternalReferenceKey(ref)
			_, included := includedExternalRefs[externalKey]
			status := "requires_confirmation"
			if included {
				status = "included"
			}
			manifest.ExternalReferences = append(manifest.ExternalReferences, SkillExternalReference{
				Path:       ref,
				SourceFile: entry.RelPath,
				Scope:      externalReferenceScope(ref),
				Included:   included,
				Status:     status,
			})
			if !included {
				manifest.Warnings = append(manifest.Warnings, SkillManifestWarning{
					Code:     "external_reference",
					Severity: "warning",
					Path:     entry.RelPath,
					Message:  ref + " is referenced outside the skill folder and was not included in this skill archive.",
				})
			}
		}
	}

	sort.Slice(manifest.ExternalReferences, func(i, j int) bool {
		if manifest.ExternalReferences[i].SourceFile == manifest.ExternalReferences[j].SourceFile {
			return manifest.ExternalReferences[i].Path < manifest.ExternalReferences[j].Path
		}
		return manifest.ExternalReferences[i].SourceFile < manifest.ExternalReferences[j].SourceFile
	})
	manifest.Summary.ExternalReferences = len(manifest.ExternalReferences)
	manifest.EnvVars = sortedKeys(envVars)
	manifest.SupportedPlatforms = sortedKeys(platforms)
	return manifest
}

func classifyManifestFile(relPath string, data []byte) string {
	clean := path.Clean(strings.TrimPrefix(strings.ReplaceAll(relPath, "\\", "/"), "/"))
	base := strings.ToLower(path.Base(clean))
	ext := strings.ToLower(path.Ext(clean))
	switch {
	case clean == "SKILL.md":
		return "entry"
	case LooksBinary(clean, data):
		return "binary"
	case isDependencyFile(base):
		return "dependency"
	case strings.HasPrefix(clean, "scripts/"), strings.HasPrefix(clean, "bin/"), isScriptExtension(ext):
		return "script"
	case strings.HasPrefix(clean, "assets/"), strings.HasPrefix(clean, "resources/"), strings.HasPrefix(clean, "templates/"), strings.HasPrefix(clean, "examples/"):
		return "resource"
	case ext == ".md" || ext == ".txt" || ext == ".rst":
		return "doc"
	case ext == ".json" || ext == ".toml" || ext == ".yaml" || ext == ".yml":
		return "config"
	default:
		return "file"
	}
}

func isDependencyFile(base string) bool {
	switch base {
	case "requirements.txt", "requirements-dev.txt", "pyproject.toml", "poetry.lock", "pipfile", "pipfile.lock",
		"environment.yml", "environment.yaml", "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
		"bun.lock", "bun.lockb", "uv.lock":
		return true
	default:
		return strings.HasPrefix(base, "requirements-") && strings.HasSuffix(base, ".txt")
	}
}

func isScriptExtension(ext string) bool {
	switch ext {
	case ".py", ".sh", ".bash", ".zsh", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".rb", ".pl":
		return true
	default:
		return false
	}
}

func looksSecretRiskPath(relPath string) bool {
	clean := strings.ToLower(path.Clean(strings.TrimPrefix(strings.ReplaceAll(relPath, "\\", "/"), "/")))
	base := path.Base(clean)
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == "id_rsa" || base == "id_ed25519" {
		return true
	}
	switch path.Ext(base) {
	case ".pem", ".key", ".p12", ".pfx":
		return true
	default:
		return strings.Contains(base, "secret") || strings.Contains(base, "credential") || strings.Contains(base, "token")
	}
}

func ExtractClaudeExternalReferences(content string) []string {
	matches := claudeExternalReferenceRE.FindAllString(content, -1)
	seen := map[string]struct{}{}
	out := []string{}
	for _, match := range matches {
		clean := strings.TrimRight(match, ".,;:")
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func ExternalAssetPathForClaudeReference(ref string) (string, bool) {
	key := claudeExternalReferenceKey(ref)
	switch {
	case strings.HasPrefix(key, "claude-tools:"):
		rest := strings.TrimPrefix(key, "claude-tools:")
		if rest == "" || rest == "." {
			return "", false
		}
		return path.Join("external/claude-tools", rest), true
	case strings.HasPrefix(key, "claude-plugins:"):
		rest := strings.TrimPrefix(key, "claude-plugins:")
		if rest == "" || rest == "." {
			return "", false
		}
		return path.Join("external/claude-plugins", rest), true
	default:
		return "", false
	}
}

func includedExternalReferenceKeys(entries []Entry) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, entry := range entries {
		key := externalAssetReferenceKey(entry.RelPath)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func externalAssetReferenceKey(relPath string) string {
	clean := path.Clean(strings.TrimPrefix(strings.ReplaceAll(relPath, "\\", "/"), "/"))
	switch {
	case strings.HasPrefix(clean, "external/claude-tools/"):
		rest := strings.TrimPrefix(clean, "external/claude-tools/")
		if rest != "" && rest != "." {
			return "claude-tools:" + rest
		}
	case strings.HasPrefix(clean, "external/claude-plugins/"):
		rest := strings.TrimPrefix(clean, "external/claude-plugins/")
		if rest != "" && rest != "." {
			return "claude-plugins:" + rest
		}
	}
	return ""
}

func claudeExternalReferenceKey(ref string) string {
	normalized := path.Clean(strings.TrimPrefix(strings.ReplaceAll(ref, "\\", "/"), "/"))
	if idx := strings.Index(normalized, ".claude/tools/"); idx >= 0 {
		rest := strings.TrimPrefix(normalized[idx:], ".claude/tools/")
		if rest != "" && rest != "." {
			return "claude-tools:" + rest
		}
	}
	if idx := strings.Index(normalized, ".claude/plugins/"); idx >= 0 {
		rest := strings.TrimPrefix(normalized[idx:], ".claude/plugins/")
		if rest != "" && rest != "." {
			return "claude-plugins:" + rest
		}
	}
	return ""
}

func externalReferenceScope(ref string) string {
	normalized := strings.ToLower(strings.ReplaceAll(ref, "\\", "/"))
	switch {
	case strings.Contains(normalized, ".claude/tools/"):
		return "claude-tools"
	case strings.Contains(normalized, ".claude/plugins/"):
		return "claude-plugins"
	default:
		return "external"
	}
}

func extractEnvVars(content string) []string {
	seen := map[string]struct{}{}
	for _, match := range envAssignmentRE.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && match[1] != "" {
			seen[match[1]] = struct{}{}
		}
	}
	for _, match := range envExpansionRE.FindAllStringSubmatch(content, -1) {
		for _, value := range match[1:] {
			if value != "" {
				seen[value] = struct{}{}
			}
		}
	}
	return sortedKeys(seen)
}

func detectSupportedPlatforms(content string) []string {
	lower := strings.ToLower(content)
	seen := map[string]struct{}{}
	for _, candidate := range []string{"claude", "claude-code", "codex", "chatgpt", "cursor", "windsurf"} {
		if strings.Contains(lower, candidate) {
			seen[candidate] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
