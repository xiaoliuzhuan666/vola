package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type cliHelpTopic struct {
	Key       string
	Summary   string
	Usage     []string
	Examples  []string
	Notes     []string
	SeeAlso   []string
	Hidden    bool
	SortOrder int
}

var cliHelpTopics = map[string]cliHelpTopic{
	"roots": {
		Key:       "roots",
		Summary:   "Understand the public Vola roots and path model.",
		Usage:     []string{"vola ls", "vola read profile/preferences", "vola read /project/demo"},
		Examples:  []string{"vola ls", "vola ls project", "vola read skill/writer/SKILL.md", "vola read secret/auth.github"},
		Notes:     []string{"Public roots are `profile`, `memory`, `project`, `skill`, `secret`, and `platform`.", "A leading `/` is optional. `project/demo` and `/project/demo` are equivalent.", "`project/<name>` is a summary view. Nested files live under paths like `project/demo/docs/notes.md`.", "`secret` is read-only in the current public command surface."},
		SeeAlso:   []string{"ls", "read", "write", "search"},
		SortOrder: 10,
	},
	"ls": {
		Key:       "ls",
		Summary:   "Browse the public Vola roots or a subtree under them.",
		Usage:     []string{"vola ls [path]"},
		Examples:  []string{"vola ls", "vola ls /", "vola ls profile", "vola ls project/demo", "vola ls skill/writer"},
		Notes:     []string{"Use `vola ls` to discover the public roots first.", "Directory output uses paths relative to the Hub root.", "A leading `/` is optional."},
		SeeAlso:   []string{"roots", "read", "search"},
		SortOrder: 20,
	},
	"read": {
		Key:       "read",
		Summary:   "Read one Vola path as text, a summary view, or a secret value.",
		Usage:     []string{"vola read <path> [--json] [--output FILE] [--local | --profile NAME | --api-base URL --token TOKEN]"},
		Examples:  []string{"vola read profile/preferences", "vola read project/demo", "vola read project/demo/docs/notes.md", "vola read skill/writer/SKILL.md", "vola read secret/auth.github"},
		Notes:     []string{"`project/<name>` returns the project summary and recent logs.", "Binary files are rejected instead of printing empty output.", "Use `--output FILE` when you want the final rendered result written locally."},
		SeeAlso:   []string{"ls", "write", "roots"},
		SortOrder: 30,
	},
	"write": {
		Key:       "write",
		Summary:   "Create or update Hub content from literal text, stdin, or a local file path.",
		Usage:     []string{"vola write <path> <content-or-file> [--literal] [--json] [--output FILE] [--local | --profile NAME | --api-base URL --token TOKEN]"},
		Examples:  []string{"vola write profile/preferences ./preferences.md", "vola write memory \"Remember this\"", "vola write project/demo/docs/notes.md ./notes.md", "vola write skill/writer/SKILL.md -"},
		Notes:     []string{"The second argument may be literal text, `-` for stdin, or a local file path.", "Use `--literal` when an argument that looks like a path should stay plain text.", "`memory` writes a new scratch memory item instead of overwriting a fixed file.", "`secret` is intentionally read-only in the current public CLI."},
		SeeAlso:   []string{"read", "log", "import"},
		SortOrder: 40,
	},
	"search": {
		Key:       "search",
		Summary:   "Search Hub content globally or under one public path scope.",
		Usage:     []string{"vola search <query> [path] [--json] [--output FILE] [--local | --profile NAME | --api-base URL --token TOKEN]"},
		Examples:  []string{"vola search migration", "vola search \"memory marker\" memory", "vola search \"launch checklist\" project/demo"},
		Notes:     []string{"When the optional path is omitted, search runs across the public Hub roots.", "`secret` search is not part of the public command surface.", "Search results are expected to be non-empty when you use them as a verification step."},
		SeeAlso:   []string{"ls", "read", "roots"},
		SortOrder: 50,
	},
	"create": {
		Key:       "create",
		Summary:   "Create a first-class Hub object.",
		Usage:     []string{"vola create project <name> [--json] [--output FILE] [--local | --profile NAME | --api-base URL --token TOKEN]"},
		Examples:  []string{"vola create project demo", "vola create project migration-notes"},
		Notes:     []string{"The category comes after the verb to match the root-directory mental model.", "The current public create surface is `project`."},
		SeeAlso:   []string{"project", "log", "read"},
		SortOrder: 60,
	},
	"log": {
		Key:       "log",
		Summary:   "Append a structured log entry to a project.",
		Usage:     []string{"vola log <path> --action ACTION --summary <text-or-file> [--tags a,b] [--literal] [--json] [--output FILE] [--local | --profile NAME | --api-base URL --token TOKEN]"},
		Examples:  []string{"vola log project/demo --action note --summary ./summary.md", "vola log project/demo --action review --summary \"Regression check complete\" --tags release,qa"},
		Notes:     []string{"`log` currently targets `project/<name>` paths.", "The summary may be literal text, stdin, or a local file path.", "Read the project again afterward to verify the log entry is present and non-empty."},
		SeeAlso:   []string{"create", "read", "write"},
		SortOrder: 70,
	},
	"import": {
		Key:     "import",
		Summary: "Bring local files or platform exports into Vola.",
		Usage: []string{
			"vola import <platform> [--dry-run] [--raw] [--zip FILE]",
			"vola import skill <local-dir> [--name NAME]",
			"vola import profile <local-file> [--category preferences|relationships|principles]",
			"vola import memory <local-file-or-dir>",
			"vola import project <local-file-or-dir> [--name NAME]",
		},
		Examples: []string{
			"vola import codex",
			"vola import claude --dry-run",
			"vola import claude --raw",
			"vola import skill ./demo-skill",
			"vola import profile ./profile.json",
			"vola import memory ./notes/",
			"vola import project ./demo-project --name imported",
		},
		Notes:     []string{"Import platform names directly, such as `import claude` or `import codex`.", "If local Git Mirror is enabled, successful imports keep syncing into that mirror automatically.", "Use `--raw` when you want the normal import plus the raw platform snapshot under `/platforms`.", "Use `import skill/profile/memory/project ...` for direct local content.", "For Claude local migration, start with `import claude --dry-run` to get a preflight inventory before writing anything."},
		SeeAlso:   []string{"write", "platform"},
		SortOrder: 80,
	},
	"token": {
		Key:       "token",
		Summary:   "Create short-lived tokens for sync or prepared skills upload workflows.",
		Usage:     []string{"vola token create --kind sync --purpose PURPOSE [--access push|pull|both] [--ttl-minutes N]", "vola token create --kind skills-upload --purpose PURPOSE [--platform PLATFORM] [--ttl-minutes N]"},
		Examples:  []string{"vola token create --kind sync --purpose backup --access both", "vola token create --kind skills-upload --purpose skills --platform claude-web"},
		Notes:     []string{"`sync` replaces the old `create_sync_token` mental model.", "`skills-upload` replaces the old `prepare_skills_upload` mental model.", "Successful output includes non-empty `token`, `expires_at`, and workflow-specific helper fields."},
		SeeAlso:   []string{"import", "sync"},
		SortOrder: 90,
	},
	"stats": {
		Key:       "stats",
		Summary:   "Show a quick summary of current Hub contents.",
		Usage:     []string{"vola stats [--json] [--output FILE] [--local | --profile NAME | --api-base URL --token TOKEN]"},
		Examples:  []string{"vola stats", "vola stats --json"},
		Notes:     []string{"Use this to confirm the Hub is non-empty after imports or writes.", "The human-readable view reports file, memory, profile, project, and skill counts."},
		SeeAlso:   []string{"status", "ls"},
		SortOrder: 100,
	},
	"platform": {
		Key:       "platform",
		Summary:   "Inspect installed platform adapters and their managed entrypoints.",
		Usage:     []string{"vola platform ls", "vola platform show <platform>"},
		Examples:  []string{"vola platform ls", "vola platform show codex", "vola platform show claude"},
		Notes:     []string{"Use `platform ls` to see which adapters are installed and connected.", "Use `platform show <platform>` to inspect config paths, entrypoints, supported domains, and embedded chat usage examples."},
		SeeAlso:   []string{"connect", "disconnect", "import"},
		SortOrder: 120,
	},
	"platform ls": {
		Key:       "platform ls",
		Summary:   "List discovered platform adapters and whether they are installed and connected.",
		Usage:     []string{"vola platform ls"},
		Examples:  []string{"vola platform ls"},
		Notes:     []string{"This is the public replacement for using root `ls` to inspect platforms.", "Output includes the adapter id, install state, connection state, and config path."},
		SeeAlso:   []string{"platform", "platform show"},
		SortOrder: 121,
	},
	"platform show": {
		Key:       "platform show",
		Summary:   "Show detailed status and routing hints for one platform adapter.",
		Usage:     []string{"vola platform show <platform>"},
		Examples:  []string{"vola platform show codex", "vola platform show claude"},
		Notes:     []string{"Use this before `connect` or `import <platform>` when you need to confirm the adapter shape.", "The `Chat usage` line is the authoritative embedded command syntax for that platform."},
		SeeAlso:   []string{"platform ls", "connect", "import"},
		SortOrder: 122,
	},
	"connect": {
		Key:       "connect",
		Summary:   "Install or refresh the Vola managed entrypoint for a platform inside the current local environment.",
		Usage:     []string{"vola connect <platform>"},
		Examples:  []string{"vola connect codex", "vola connect claude"},
		Notes:     []string{"This command targets the current local environment; in isolated tests it should run under a temporary HOME/XDG root.", "A successful result reports the managed entrypoint path and embedded chat usage examples."},
		SeeAlso:   []string{"platform show", "disconnect"},
		SortOrder: 130,
	},
	"disconnect": {
		Key:       "disconnect",
		Summary:   "Remove a Vola managed platform entrypoint and stored connection metadata.",
		Usage:     []string{"vola disconnect <platform>"},
		Examples:  []string{"vola disconnect codex", "vola disconnect claude"},
		Notes:     []string{"Use this when you want to remove the Vola managed skill or command file from the current environment.", "This is operational cleanup, not a public Hub data command."},
		SeeAlso:   []string{"connect", "platform show"},
		SortOrder: 140,
	},
	"export": {
		Key:       "export",
		Summary:   "Stage platform-oriented export materials from the current local Hub state.",
		Usage:     []string{"vola export <platform> [--output DIR]"},
		Examples:  []string{"vola export codex --output ./codex-export", "vola export claude --output ./claude-export"},
		Notes:     []string{"Use this when you want platform-shaped export materials, not a Git mirror of the Hub itself.", "If the user wants a repo mirror of the Hub, use the Git Mirror workflow in the dashboard instead."},
		SeeAlso:   []string{"platform"},
		SortOrder: 150,
	},
	"status": {
		Key:       "status",
		Summary:   "Show whether the local daemon, current target, and configured storage are ready to use.",
		Usage:     []string{"vola status"},
		Examples:  []string{"vola status"},
		Notes:     []string{"This is the quickest operational readiness check.", "The output reports local daemon state, local storage backend, current target, and hosted profile details when selected."},
		SeeAlso:   []string{"doctor", "stats"},
		SortOrder: 160,
	},
	"login": {
		Key:       "login",
		Summary:   "Open the browser and sign in to a hosted Vola profile.",
		Usage:     []string{"vola login [--profile NAME] [--api-base URL]", "vola login --profile official --api-base https://vola.ai"},
		Examples:  []string{"vola login", "vola login --profile official", "vola login --profile staging --api-base https://vola.ai"},
		Notes:     []string{"This is the primary hosted login entrypoint.", "The CLI opens a browser, completes OAuth, stores an access token plus refresh token, and switches the current target to that profile.", "Use `--token` only when you already have a bearer token and want to save it manually."},
		SeeAlso:   []string{"profiles", "use", "whoami"},
		SortOrder: 171,
	},
	"logout": {
		Key:       "logout",
		Summary:   "Clear the saved hosted session for one profile.",
		Usage:     []string{"vola logout [--profile NAME]"},
		Examples:  []string{"vola logout", "vola logout --profile official"},
		Notes:     []string{"If you log out the currently selected hosted profile, the CLI falls back to the local target."},
		SeeAlso:   []string{"login", "profiles", "use"},
		SortOrder: 172,
	},
	"use": {
		Key:       "use",
		Summary:   "Switch the default target between local and a saved hosted profile.",
		Usage:     []string{"vola use <local|profile>"},
		Examples:  []string{"vola use local", "vola use official"},
		Notes:     []string{"Hub commands and hosted-aware sync commands follow the current target unless you pass `--local`, `--profile`, or explicit `--api-base --token` overrides."},
		SeeAlso:   []string{"login", "profiles", "whoami", "status"},
		SortOrder: 173,
	},
	"whoami": {
		Key:       "whoami",
		Summary:   "Show the active authentication identity for the resolved target.",
		Usage:     []string{"vola whoami [--local | --profile NAME | --api-base URL --token TOKEN]"},
		Examples:  []string{"vola whoami", "vola whoami --local", "vola whoami --profile official"},
		Notes:     []string{"Use this to confirm which target, user, auth mode, and scopes the CLI will use before running writes or sync operations."},
		SeeAlso:   []string{"status", "profiles", "use"},
		SortOrder: 174,
	},
	"profiles": {
		Key:       "profiles",
		Summary:   "List saved hosted profiles and show which target is active.",
		Usage:     []string{"vola profiles"},
		Examples:  []string{"vola profiles"},
		Notes:     []string{"The list includes `local` plus all saved hosted profiles, along with auth mode, scope summary, and expiry status."},
		SeeAlso:   []string{"login", "logout", "use", "whoami"},
		SortOrder: 175,
	},
	"browse": {
		Key:       "browse",
		Summary:   "Open the local Vola dashboard or print its authenticated URL.",
		Usage:     []string{"vola browse [--print-url] [/route]"},
		Examples:  []string{"vola browse", "vola browse --print-url /data/files"},
		Notes:     []string{"Use `--print-url` in scripts or terminal-only environments.", "The route is resolved relative to the local dashboard root."},
		SeeAlso:   []string{"status"},
		SortOrder: 170,
	},
	"doctor": {
		Key:       "doctor",
		Summary:   "Run a concise local readiness diagnostic.",
		Usage:     []string{"vola doctor"},
		Examples:  []string{"vola doctor"},
		Notes:     []string{"Use this when `status` is not enough and you want pointed next-step diagnostics."},
		SeeAlso:   []string{"status"},
		SortOrder: 180,
	},
	"daemon": {
		Key:       "daemon",
		Summary:   "Inspect or manage the local Vola daemon process.",
		Usage:     []string{"vola daemon status", "vola daemon logs [--tail N]", "vola daemon stop"},
		Examples:  []string{"vola daemon status", "vola daemon logs --tail 50", "vola daemon stop"},
		Notes:     []string{"The public Hub data commands start the local daemon on demand when needed.", "Use this when you explicitly want daemon-level diagnostics or cleanup."},
		SeeAlso:   []string{"status", "doctor"},
		SortOrder: 190,
	},
	"sync": {
		Key:       "sync",
		Summary:   "Manage bundle-style sync workflows against the current target or an archive transport.",
		Usage:     []string{"vola sync <subcommand>"},
		Examples:  []string{"vola sync export --source ./skills --format archive -o backup.ndrvz", "vola sync push --bundle backup.ndrvz", "vola sync pull --format archive -o restore.ndrvz"},
		Notes:     []string{"`sync` is the bundle transfer surface and is separate from the root-directory Hub commands.", "Authentication and default target selection now live at the top level via `vola login`, `vola use`, and `vola whoami`.", "Use `vola token create --kind sync` when you need an ephemeral sync token for a one-off push or pull."},
		SeeAlso:   []string{"login", "use", "whoami", "token"},
		SortOrder: 200,
	},
	"server": {
		Key:       "server",
		Summary:   "Start the standalone Vola HTTP server.",
		Usage:     []string{"vola server [flags]"},
		Examples:  []string{"vola server --listen 127.0.0.1:42690 --local-mode"},
		Notes:     []string{"This is mostly for explicit server operation, not day-to-day local CLI use."},
		SeeAlso:   []string{"mcp"},
		SortOrder: 220,
	},
	"mcp": {
		Key:       "mcp",
		Summary:   "Run the Vola MCP server over stdio.",
		Usage:     []string{"vola mcp stdio [flags]"},
		Examples:  []string{"vola mcp stdio --token-env VOLA_TOKEN"},
		Notes:     []string{"This is the low-level MCP entrypoint used by managed platform integrations."},
		SeeAlso:   []string{"server", "connect"},
		SortOrder: 230,
	},
}

var cliHelpAliases = map[string]string{
	"root":         "",
	"overview":     "",
	"paths":        "roots",
	"path":         "roots",
	"profile":      "roots",
	"memory":       "roots",
	"memories":     "roots",
	"project":      "roots",
	"projects":     "roots",
	"skill":        "roots",
	"skills":       "roots",
	"secret":       "roots",
	"secrets":      "roots",
	"platforms":    "platform",
	"list":         "ls",
	"token create": "token",
}

func runHelp(args []string) int {
	if len(args) == 0 {
		printRootUsage()
		return 0
	}
	if printHelpTopic(strings.Join(args, " ")) {
		return 0
	}
	fmt.Fprintf(os.Stderr, "unknown help topic %q\n\n", strings.Join(args, " "))
	fmt.Fprintf(os.Stderr, "available topics: %s\n\n", strings.Join(helpTopicsList(), ", "))
	printRootUsage()
	return 2
}

func printRootUsage() {
	fmt.Print(renderCLIText(`Vola

Root-directory command surface for local and hosted Vola data.

Mental model:
  - Start at the Hub root with vola ls
  - Public roots: profile, memory, project, skill, secret, platform
  - A leading / is optional. project/demo and /project/demo are equivalent.
  - Recommended command name: neu. Compatibility aliases: vola, vol, neudrive, xlzdrive.

Public commands:
  vola help [topic]                              Show root help or a topic-specific guide
  vola ls [path]                                 Browse public roots or a subtree
  vola read <path>                               Read a file, summary view, or secret
  vola write <path> <content-or-file>            Create or update Hub content
  vola search <query> [path]                     Search Hub content
  vola create <category> <name>                  Create a first-class Hub object
  vola log <path> --action ACTION --summary ...  Append a project log entry
  vola import <platform-or-category> ...         Import local or platform data
  vola token create --kind sync|skills-upload    Create a short-lived workflow token
  vola stats                                     Show a quick Hub summary

Operational commands:
  vola platform ls
  vola platform show <platform>
  vola connect <platform>
  vola disconnect <platform>
  vola export <platform> [--output DIR]
  vola browse [/route]
  vola status
  vola doctor
  vola login [--profile NAME]
  vola logout [--profile NAME]
  vola use <local|profile>
  vola whoami
  vola profiles
  vola daemon status|stop|logs
  vola sync <subcommand>
  vola server [flags]
  vola mcp stdio [flags]

Examples:
  vola ls
  vola read profile/preferences
  vola write memory "Remember this"
  vola create project demo
  vola import skill ./demo-skill

More help:
  vola help roots
  vola help write
  vola help import
`))
}

func printHelpTopic(raw string) bool {
	topic, ok := lookupHelpTopic(raw)
	if !ok {
		return false
	}
	fmt.Printf("%s\n\n", topicHeading(topic.Key))
	fmt.Printf("%s\n\n", renderCLIText(topic.Summary))
	if len(topic.Usage) > 0 {
		fmt.Println("Usage:")
		for _, line := range topic.Usage {
			fmt.Printf("  %s\n", renderCLIText(line))
		}
		fmt.Println()
	}
	if len(topic.Examples) > 0 {
		fmt.Println("Examples:")
		for _, line := range topic.Examples {
			fmt.Printf("  %s\n", renderCLIText(line))
		}
		fmt.Println()
	}
	if len(topic.Notes) > 0 {
		fmt.Println("Notes:")
		for _, line := range topic.Notes {
			fmt.Printf("  - %s\n", renderCLIText(line))
		}
		fmt.Println()
	}
	if len(topic.SeeAlso) > 0 {
		fmt.Printf("See also: %s\n", strings.Join(topic.SeeAlso, ", "))
	}
	return true
}

func topicHeading(key string) string {
	if key == "roots" {
		return "Vola Path Model"
	}
	return fmt.Sprintf("%s %s", rootCommand(), key)
}

func lookupHelpTopic(raw string) (cliHelpTopic, bool) {
	key := normalizeHelpTopic(raw)
	if alias, ok := cliHelpAliases[key]; ok {
		key = alias
	}
	if key == "" {
		return cliHelpTopic{}, false
	}
	topic, ok := cliHelpTopics[key]
	return topic, ok
}

func normalizeHelpTopic(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, rootCommand()+" ")
	raw = strings.TrimPrefix(raw, "vola ")
	raw = strings.TrimPrefix(raw, "neu ")
	raw = strings.TrimPrefix(raw, "/")
	raw = strings.Join(strings.Fields(raw), " ")
	return raw
}

func isExplicitHelpRequest(args []string) bool {
	if isHelpArg(args) {
		return true
	}
	return containsFlag(args, "--help", "-h")
}

func helpTopicsList() []string {
	topics := make([]cliHelpTopic, 0, len(cliHelpTopics))
	for _, topic := range cliHelpTopics {
		if topic.Hidden {
			continue
		}
		topics = append(topics, topic)
	}
	sort.Slice(topics, func(i, j int) bool {
		if topics[i].SortOrder == topics[j].SortOrder {
			return topics[i].Key < topics[j].Key
		}
		return topics[i].SortOrder < topics[j].SortOrder
	})
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		out = append(out, topic.Key)
	}
	return out
}
