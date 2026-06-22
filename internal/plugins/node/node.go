package node

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/turing/shack/internal/plugins"
	"gopkg.in/yaml.v3"
)

func New() plugins.Plugin { return &node{} }

func init() { plugins.Register(New()) }

type node struct{}

func (*node) Name() string      { return "node" }
func (*node) Markers() []string { return []string{"package.json"} }

// PackageJSON is the subset of package.json fields the plugin reads.
// Exported so tests in this package can construct fixtures directly
// without round-tripping through JSON.
type PackageJSON struct {
	Name             string            `json:"name"`
	Scripts          map[string]string `json:"scripts"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
	Workspaces       interface{}       `json:"workspaces"`     // string[] or {packages:[]}
	PackageManager   string            `json:"packageManager"` // e.g. "pnpm@8.6.0"
}

// serverFrameworks lists deps whose presence makes a project server-eligible.
// Membership is sufficient: any one of these in any of the dependency maps
// (root or workspace) flips the gate.
var serverFrameworks = map[string]bool{
	"fastify":           true,
	"next":              true,
	"vite":              true,
	"nuxt":              true,
	"astro":             true,
	"@nestjs/core":      true,
	"express":           true,
	"koa":               true,
	"hono":              true,
	"@hono/node-server": true,
}

// frameworkBinaries maps binary names used in scripts to their canonical
// package name (some frameworks install under a different binary name).
var frameworkBinaries = map[string]string{
	"fastify":  "fastify",
	"next":     "next",
	"vite":     "vite",
	"nuxt":     "nuxt",
	"astro":    "astro",
	"nestjs":   "@nestjs/core",
	"express":  "express",
	"koa":      "koa",
	"hono":     "hono",
	"wrangler": "wrangler", // not in serverFrameworks dep gate but checked by bin existence
}

// serverEligibilityPackages is the set used by entryFileBindsServer when
// scanning import statements.
var serverEligibilityPackages = []string{
	"fastify", "next", "vite", "nuxt", "astro", "@nestjs/core",
	"express", "koa", "hono", "@hono/node-server",
}

// runtimeBinaries are binaries that execute a user-written entry file.
var runtimeBinaries = map[string]bool{
	"node": true,
	"tsx":  true,
	"bun":  true,
	"deno": true,
}

// entryFileExtensions are extensions that identify JS/TS entry files.
var entryFileExtRe = regexp.MustCompile(`\.(ts|tsx|js|mjs|cjs)$`)

// serverImportRe matches import/require of any server-eligibility package on a single line.
var serverImportRe = regexp.MustCompile(
	`(?:from|require\()\s*['"]` +
		`(fastify|next|vite|nuxt|astro|@nestjs/core|express|koa|hono|@hono/node-server)` +
		`['"]`,
)

// serverCallRe matches .listen(, createServer(, or serve( calls.
var serverCallRe = regexp.MustCompile(`(?:\.listen\(|createServer\(|serve\()`)

// relativeImportRe matches relative import/require/dynamic-import specifiers,
// including side-effect imports (import './foo'). Group 1 captures the path.
var relativeImportRe = regexp.MustCompile(
	`(?:from|require\(|import\(|import\s+)\s*['"](\.[./][^'"]*?)['"]`,
)

// relativeImportExtensions is the ordered list of extensions to try when
// resolving an import specifier that has no extension or whose extension
// doesn't exist on disk.
var relativeImportExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

// stripJSExt removes a .js, .mjs, or .cjs suffix from p (TS source files are
// often imported with these suffixes in ESM output).
func stripJSExt(p string) string {
	for _, ext := range []string{".js", ".mjs", ".cjs"} {
		if strings.HasSuffix(p, ext) {
			return p[:len(p)-len(ext)]
		}
	}
	return p
}

// resolveRelativeImport resolves an import specifier relative to the directory
// of the importing file, constrained to stay within projectDir.
// It returns the absolute path of the resolved file, or "" if not found or
// if the resolved path would escape projectDir.
func resolveRelativeImport(importingDir, specifier, projectDir string) string {
	base := filepath.Join(importingDir, filepath.FromSlash(specifier))

	// Guard: skip paths that escape projectDir.
	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return ""
	}

	tryPath := func(p string) string {
		abs, err := filepath.Abs(p)
		if err != nil {
			return ""
		}
		rel, err := filepath.Rel(absProject, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return ""
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			return ""
		}
		return abs
	}

	// 1. Try as-is.
	if p := tryPath(base); p != "" {
		return p
	}

	// 2. Strip JS-style extension and try with each candidate extension.
	stripped := stripJSExt(base)
	for _, ext := range relativeImportExtensions {
		if p := tryPath(stripped + ext); p != "" {
			return p
		}
	}
	// If base already had a recognised extension that didn't exist, also try
	// the original base with the other extensions.
	if stripped != base {
		for _, ext := range relativeImportExtensions {
			if p := tryPath(base + ext); p != "" {
				return p
			}
		}
	}

	// 3. If the specifier (or stripped version) resolves to a directory, try
	// index files inside it.
	for _, candidate := range []string{base, stripped} {
		absCandidate, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(absCandidate)
		if err != nil || !info.IsDir() {
			continue
		}
		for _, ext := range relativeImportExtensions {
			if p := tryPath(filepath.Join(absCandidate, "index"+ext)); p != "" {
				return p
			}
		}
	}

	return ""
}

// bindsServer is the recursive implementation behind entryFileBindsServer.
// It reads up to 500 lines of the file at path, checks for direct server
// signals, and (when depth < maxBindsServerDepth) recursively checks any
// relative imports found in the file.
const maxBindsServerDepth = 3

func bindsServer(path string, depth int, visited map[string]bool, projectDir string) bool {
	if depth > maxBindsServerDepth {
		return false
	}
	if visited[path] {
		return false
	}
	visited[path] = true

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	importingDir := filepath.Dir(path)
	var relImports []string

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		if lineCount > 500 {
			break
		}
		if serverImportRe.MatchString(line) {
			return true
		}
		if serverCallRe.MatchString(line) {
			return true
		}
		// Collect relative imports for recursive scanning.
		if depth < maxBindsServerDepth {
			if m := relativeImportRe.FindStringSubmatch(line); m != nil {
				specifier := m[1]
				if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") {
					relImports = append(relImports, specifier)
				}
			}
		}
	}

	// Recurse into collected relative imports.
	for _, spec := range relImports {
		resolved := resolveRelativeImport(importingDir, spec, projectDir)
		if resolved == "" {
			continue
		}
		if bindsServer(resolved, depth+1, visited, projectDir) {
			return true
		}
	}

	return false
}

// entryFileBindsServer reads a JS/TS entry file (and follows relative imports
// up to 3 hops deep) to determine whether the file or any transitive local
// import contains server-framework usage or a server-binding call.
func entryFileBindsServer(path string, projectDir string) bool {
	return bindsServer(path, 0, map[string]bool{}, projectDir)
}

func readPkg(dir string) (PackageJSON, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return PackageJSON{}, err
	}
	var p PackageJSON
	if err := json.Unmarshal(raw, &p); err != nil {
		return PackageJSON{}, fmt.Errorf("parse package.json in %s: %w", dir, err)
	}
	return p, nil
}

// isServerEligible returns true iff any of root or workspacePkgs depends
// on a known server framework (in dependencies, devDependencies, or
// peerDependencies).
func isServerEligible(root PackageJSON, workspacePkgs []PackageJSON) bool {
	check := func(p PackageJSON) bool {
		for _, m := range []map[string]string{p.Dependencies, p.DevDependencies, p.PeerDependencies} {
			for name := range m {
				if serverFrameworks[name] {
					return true
				}
			}
		}
		return false
	}
	if check(root) {
		return true
	}
	for _, w := range workspacePkgs {
		if check(w) {
			return true
		}
	}
	return false
}

// splitTopLevel splits s on the operators &&, ||, and ;. It does not
// handle quotes; shell command strings in package.json are assumed to be
// simple enough that this is sufficient.
func splitTopLevel(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch {
		case i+1 < len(s) && s[i] == '&' && s[i+1] == '&':
			out = append(out, s[start:i])
			i++
			start = i + 1
		case i+1 < len(s) && s[i] == '|' && s[i+1] == '|':
			out = append(out, s[start:i])
			i++
			start = i + 1
		case s[i] == ';':
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// envVarRe matches leading KEY=value assignments (e.g., NODE_ENV=production).
var envVarRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=\S*\s*`)

// parseFirstToken strips leading env-var assignments from a trimmed segment
// and returns the leading binary token and the remaining args as a slice.
// If the leading token is "cd", it also strips it and the directory that
// follows, returning the next real binary and the cdDir.
// Returns binary="", cdDir="", args=nil for empty/env-only segments.
func parseFirstToken(segment string) (binary string, cdDir string, args []string) {
	s := strings.TrimSpace(segment)
	// Strip leading env-var assignments.
	for envVarRe.MatchString(s) {
		loc := envVarRe.FindStringIndex(s)
		s = s[loc[1]:]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", nil
	}
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "", "", nil
	}
	// Handle leading `cd <dir>` prefix.
	if parts[0] == "cd" && len(parts) >= 2 {
		return "", parts[1], nil
	}
	return parts[0], "", parts[1:]
}

// isFrameworkBinary checks whether binary is a known framework binary
// installed in node_modules/.bin, and applies per-framework subcommand rules.
// rootDir is the project root where node_modules lives.
func isFrameworkBinary(rootDir, binary string, args []string) bool {
	// Check that the binary is actually installed.
	binPath := filepath.Join(rootDir, "node_modules", ".bin", binary)
	if _, err := os.Stat(binPath); err != nil {
		return false
	}

	subcommand := ""
	if len(args) > 0 {
		// First positional arg (skip flag-like args starting with -)
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				subcommand = a
				break
			}
		}
	}

	switch binary {
	case "vite":
		// vite alone (dev) or vite dev/serve — not vite build or vite preview
		return subcommand == "" || subcommand == "dev" || subcommand == "serve"
	case "next":
		return subcommand == "dev" || subcommand == "start"
	case "nuxt":
		return subcommand == "dev" || subcommand == "start"
	case "astro":
		return subcommand == "dev" || subcommand == "start"
	case "wrangler":
		return subcommand == "dev"
	case "fastify", "express", "koa", "hono", "nestjs":
		// No subcommand model; binary itself is the server.
		return true
	}
	// Unknown framework binary that happens to be in node_modules/.bin.
	return false
}

// findEntryFile finds the last positional arg in args that looks like
// a JS/TS file path. It resolves the path relative to rootDir and returns
// the absolute path if the file exists, or "" otherwise.
func findEntryFile(args []string, rootDir string) string {
	candidate := ""
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		// Accept strings with a recognised extension.
		if entryFileExtRe.MatchString(a) {
			candidate = a
		}
	}
	if candidate == "" {
		return ""
	}
	abs := filepath.Join(rootDir, candidate)
	if _, err := os.Stat(abs); err != nil {
		return ""
	}
	return abs
}

// recurseScript looks up scriptName in rootDir/package.json#scripts and
// calls isServerScript on its command. Depth-limited to prevent cycles.
func recurseScript(rootDir, scriptName string, depth int) bool {
	pkg, err := readPkg(rootDir)
	if err != nil {
		return false
	}
	cmd, ok := pkg.Scripts[scriptName]
	if !ok {
		return false
	}
	return isServerScript(rootDir, cmd, depth+1)
}

// recurseWorkspace reads <rootDir>/<dir>/package.json and calls
// isServerScript on the named script's command.
func recurseWorkspace(rootDir, dir, scriptName string, depth int) bool {
	wsDir := filepath.Join(rootDir, dir)
	return recurseScript(wsDir, scriptName, depth)
}

// isServerScript returns true if the script command (from package.json#scripts)
// represents a server invocation. rootDir is the project root. depth is used
// for cycle detection (max depth 5).
func isServerScript(rootDir, scriptCmd string, depth int) bool {
	if depth >= 5 {
		return false
	}

	segments := splitTopLevel(scriptCmd)
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		binary, cdDir, args := parseFirstToken(seg)

		// Case C (partial): leading `cd <dir>` means workspace recursion.
		// The rest of the segment should be handled as a workspace invocation.
		if cdDir != "" {
			// After cd <dir>, the segment ends — we need the next && segment
			// to find the pnpm/npm/yarn call. But since cd and the next command
			// appear in the same segment only with &&, they've already been
			// split. So cdDir signals nothing actionable here unless the full
			// segment is `cd <dir> && pnpm <script>`, which splitTopLevel would
			// split into two parts. We need to look at the FULL original segment
			// for the cd+command pattern.
			//
			// Actually: splitTopLevel splits on && so `cd web && pnpm dev` becomes
			// ["cd web ", " pnpm dev"]. The cd segment has cdDir="web", binary="",
			// and the pnpm segment is handled separately. But we need the cdDir
			// context for the pnpm call in the next segment. So we look at pairs.
			// Instead, handle `cd <dir>` by scanning the segment list for the
			// pattern manually below.
			_ = cdDir
			continue
		}

		if binary == "" {
			continue
		}

		// Case C: package-manager recursion.
		if binary == "pnpm" || binary == "npm" || binary == "yarn" {
			if len(args) == 0 {
				continue
			}
			var scriptName string
			if args[0] == "run" && len(args) >= 2 {
				scriptName = args[1]
			} else if binary == "pnpm" && args[0] != "install" && args[0] != "add" && args[0] != "remove" {
				// pnpm auto-runs scripts by name
				scriptName = args[0]
			}
			if scriptName != "" {
				if recurseScript(rootDir, scriptName, depth) {
					return true
				}
			}
			continue
		}

		// Case A: direct framework binary.
		if isFrameworkBinary(rootDir, binary, args) {
			return true
		}

		// Case B: runtime invoking a user entry file.
		if runtimeBinaries[binary] {
			entryFile := findEntryFile(args, rootDir)
			if entryFile != "" && entryFileBindsServer(entryFile, rootDir) {
				return true
			}
		}
	}

	// Second pass: handle `cd <dir> && <pkgmgr> <script>` by scanning adjacent
	// segments for the (cd, pkgmgr) pair.
	segments = splitTopLevel(scriptCmd)
	for i := 0; i < len(segments)-1; i++ {
		seg0 := strings.TrimSpace(segments[i])
		seg1 := strings.TrimSpace(segments[i+1])
		b0, dir0, _ := parseFirstToken(seg0)
		if b0 != "" || dir0 == "" {
			continue
		}
		// seg0 is `cd <dir>`.
		b1, _, args1 := parseFirstToken(seg1)
		if b1 != "pnpm" && b1 != "npm" && b1 != "yarn" {
			continue
		}
		var scriptName string
		if len(args1) == 0 {
			continue
		}
		if args1[0] == "run" && len(args1) >= 2 {
			scriptName = args1[1]
		} else if b1 == "pnpm" && args1[0] != "install" && args1[0] != "add" && args1[0] != "remove" {
			scriptName = args1[0]
		}
		if scriptName == "" {
			continue
		}
		if recurseWorkspace(rootDir, dir0, scriptName, depth) {
			return true
		}
	}

	return false
}

// detectPackageManager returns "pnpm", "yarn", "npm", or "bun".
// Priority: package.json#packageManager → lockfiles → pnpm-workspace.yaml
// → npm fallback.
func detectPackageManager(projectDir string) string {
	// 1. package.json#packageManager (corepack-style explicit declaration).
	if pkg, err := readPkg(projectDir); err == nil && pkg.PackageManager != "" {
		spec := pkg.PackageManager
		if i := strings.IndexByte(spec, '@'); i >= 0 {
			spec = spec[:i]
		}
		switch spec {
		case "pnpm", "yarn", "npm", "bun":
			return spec
		}
	}
	// 2. Lockfiles in priority order.
	for _, c := range []struct {
		file string
		pm   string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"package-lock.json", "npm"},
		// 3. pnpm-workspace.yaml is pnpm-only; covers monorepos before
		// first install.
		{"pnpm-workspace.yaml", "pnpm"},
	} {
		if _, err := os.Stat(filepath.Join(projectDir, c.file)); err == nil {
			return c.pm
		}
	}
	return "npm"
}

// rootCmd builds the run-script invocation for the detected package
// manager at the project root.
func rootCmd(pm, label string) string {
	switch pm {
	case "yarn":
		return "yarn " + label
	case "bun":
		return "bun run " + label
	case "npm":
		return "npm run " + label
	default: // pnpm
		return "pnpm " + label
	}
}

// workspaceCmd builds the workspace-filtered run invocation.
func workspaceCmd(pm, wsName, label string) string {
	switch pm {
	case "yarn":
		// yarn 1.x: `yarn workspace <name> <script>`. yarn 2+: same.
		return fmt.Sprintf("yarn workspace %s %s", wsName, label)
	case "bun":
		return fmt.Sprintf("bun run --filter %s %s", wsName, label)
	case "npm":
		return fmt.Sprintf("npm run %s --workspace=%s", label, wsName)
	default: // pnpm
		return fmt.Sprintf("pnpm --filter %s %s", wsName, label)
	}
}

// Suggest emits one Suggestion per script that survives the two-pass
// filter: (1) the project must be server-eligible (root or any workspace
// pkg depends on a known server framework); (2) the script's command
// must classify as a server invocation via code-level introspection.
// The package manager is detected from lockfiles (pnpm-lock.yaml,
// yarn.lock, bun.lockb, package-lock.json) and the resulting Cmd uses
// the correct invocation form.
func (*node) Suggest(projectDir string) ([]plugins.Suggestion, error) {
	root, err := readPkg(projectDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	wsPaths, err := workspacePackagePaths(projectDir, root)
	if err != nil {
		return nil, err
	}
	var workspacePkgs []PackageJSON
	for _, wsDir := range wsPaths {
		ws, err := readPkg(wsDir)
		if err != nil {
			continue
		}
		workspacePkgs = append(workspacePkgs, ws)
	}

	if !isServerEligible(root, workspacePkgs) {
		return nil, nil
	}

	pm := detectPackageManager(projectDir)
	var out []plugins.Suggestion
	for label, scriptBody := range root.Scripts {
		if isServerScript(projectDir, scriptBody, 0) {
			out = append(out, plugins.Suggestion{
				Label: label,
				Cmd:   rootCmd(pm, label),
				Body:  scriptBody,
			})
		}
	}
	for i, ws := range workspacePkgs {
		if ws.Name == "" {
			continue
		}
		wsDir := wsPaths[i]
		for label, scriptBody := range ws.Scripts {
			if isServerScript(wsDir, scriptBody, 0) {
				out = append(out, plugins.Suggestion{
					Label: ws.Name + ":" + label,
					Cmd:   workspaceCmd(pm, ws.Name, label),
					Body:  scriptBody,
				})
			}
		}
	}
	return out, nil
}

// workspacePackagePaths resolves workspace globs from pnpm-workspace.yaml
// (preferred) or package.json#workspaces (fallback). Returns absolute
// directories.
func workspacePackagePaths(projectDir string, root PackageJSON) ([]string, error) {
	patterns, err := pnpmWorkspaces(projectDir)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		patterns = npmStyleWorkspaces(root)
	}
	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	seen := map[string]bool{}
	for _, pat := range patterns {
		joined := filepath.Join(projectDir, pat)
		// Guard: skip patterns that resolve outside projectDir.
		rel, err := filepath.Rel(absProject, joined)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		matches, err := filepath.Glob(joined)
		if err != nil {
			continue
		}
		for _, m := range matches {
			// Guard: verify each resolved match is also inside projectDir.
			absMatch, err := filepath.Abs(m)
			if err != nil {
				continue
			}
			relMatch, err := filepath.Rel(absProject, absMatch)
			if err != nil || strings.HasPrefix(relMatch, "..") {
				continue
			}
			info, err := os.Stat(m)
			if err != nil || !info.IsDir() {
				continue
			}
			if !seen[m] {
				seen[m] = true
				dirs = append(dirs, m)
			}
		}
	}
	return dirs, nil
}

func pnpmWorkspaces(projectDir string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(projectDir, "pnpm-workspace.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse pnpm-workspace.yaml: %w", err)
	}
	return doc.Packages, nil
}

func npmStyleWorkspaces(p PackageJSON) []string {
	switch w := p.Workspaces.(type) {
	case []interface{}:
		out := make([]string, 0, len(w))
		for _, e := range w {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case map[string]interface{}:
		if pkgs, ok := w["packages"].([]interface{}); ok {
			out := make([]string, 0, len(pkgs))
			for _, e := range pkgs {
				if s, ok := e.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}
