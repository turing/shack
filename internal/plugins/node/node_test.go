package node

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNodeMarkers(t *testing.T) {
	got := New().Markers()
	if len(got) != 1 || got[0] != "package.json" {
		t.Errorf("got %v, want [package.json]", got)
	}
}

// TestIsServerEligible exercises the dependency gate.
func TestIsServerEligible(t *testing.T) {
	cases := []struct {
		name string
		root PackageJSON
		want bool
	}{
		{"fastify", PackageJSON{Dependencies: map[string]string{"fastify": "1"}}, true},
		{"vite-dev-dep", PackageJSON{DevDependencies: map[string]string{"vite": "5"}}, true},
		{"tsx-vitest-only", PackageJSON{DevDependencies: map[string]string{"tsx": "4", "vitest": "1"}}, false},
		{"empty", PackageJSON{}, false},
	}
	for _, c := range cases {
		if got := isServerEligible(c.root, nil); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// helpers to set up tempdir with node_modules/.bin stubs and entry files.

func mkBin(t *testing.T, dir, name string) {
	t.Helper()
	binDir := filepath.Join(dir, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(binDir, name))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func mkFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// --- Case A: direct framework binary tests ---

func TestIsServerScriptDirectViteAlone(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "vite")
	if !isServerScript(dir, "vite", 0) {
		t.Error("expected vite alone to be a server script")
	}
}

func TestIsServerScriptDirectViteDev(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "vite")
	if !isServerScript(dir, "vite dev", 0) {
		t.Error("expected 'vite dev' to be a server script")
	}
}

func TestIsServerScriptDirectViteBuild(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "vite")
	if isServerScript(dir, "vite build", 0) {
		t.Error("expected 'vite build' NOT to be a server script")
	}
}

func TestIsServerScriptDirectVitePreview(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "vite")
	if isServerScript(dir, "vite preview", 0) {
		t.Error("expected 'vite preview' NOT to be a server script")
	}
}

func TestIsServerScriptNextDev(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "next")
	if !isServerScript(dir, "next dev", 0) {
		t.Error("expected 'next dev' to be a server script")
	}
}

func TestIsServerScriptNextStart(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "next")
	if !isServerScript(dir, "next start", 0) {
		t.Error("expected 'next start' to be a server script")
	}
}

func TestIsServerScriptNextBuild(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "next")
	if isServerScript(dir, "next build", 0) {
		t.Error("expected 'next build' NOT to be a server script")
	}
}

func TestIsServerScriptNextLint(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "next")
	if isServerScript(dir, "next lint", 0) {
		t.Error("expected 'next lint' NOT to be a server script")
	}
}

func TestIsServerScriptWranglerDev(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "wrangler")
	if !isServerScript(dir, "wrangler dev", 0) {
		t.Error("expected 'wrangler dev' to be a server script")
	}
}

func TestIsServerScriptWranglerDeploy(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "wrangler")
	if isServerScript(dir, "wrangler deploy", 0) {
		t.Error("expected 'wrangler deploy' NOT to be a server script")
	}
}

// Framework binary not installed → not a server.
func TestIsServerScriptFrameworkBinNotInstalled(t *testing.T) {
	dir := t.TempDir()
	// No node_modules/.bin/vite created.
	if isServerScript(dir, "vite", 0) {
		t.Error("expected 'vite' NOT to be a server script when binary not installed")
	}
}

// --- Case B: runtime invoking user code ---

const fastifyEntryContent = `import fastify from 'fastify'

const app = fastify()
app.get('/', async () => ({ hello: 'world' }))
app.listen({ port: 3000 })
`

const nonServerEntryContent = `import fs from 'fs'

console.log(fs.readFileSync('README.md', 'utf8'))
`

func TestIsServerScriptTsxWithFastifyEntry(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "tsx")
	mkFile(t, dir, "src/server.ts", fastifyEntryContent)
	if !isServerScript(dir, "tsx watch src/server.ts", 0) {
		t.Error("expected tsx watch src/server.ts to be a server script")
	}
}

func TestIsServerScriptTsxWithNonServerEntry(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "tsx")
	mkFile(t, dir, "src/cli.ts", nonServerEntryContent)
	if isServerScript(dir, "tsx src/cli.ts", 0) {
		t.Error("expected tsx src/cli.ts NOT to be a server script")
	}
}

func TestIsServerScriptNodeDistMissing(t *testing.T) {
	dir := t.TempDir()
	// dist/server.js does NOT exist.
	if isServerScript(dir, "node dist/server.js", 0) {
		t.Error("expected node dist/server.js NOT to be a server script when file missing")
	}
}

func TestIsServerScriptNodeWithListenCall(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "dist/server.js", "const http = require('http'); const s = http.createServer(); s.listen(3000);\n")
	if !isServerScript(dir, "node dist/server.js", 0) {
		t.Error("expected node dist/server.js to be a server script when file has .listen(")
	}
}

func TestIsServerScriptTsxEnvFlagsWithServerEntry(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "tsx")
	mkFile(t, dir, "src/index.ts", fastifyEntryContent)
	// Flags like --env-file=.env should be skipped when finding entry file.
	if !isServerScript(dir, "tsx watch --env-file=.env --env-file-if-exists=.env.local src/index.ts", 0) {
		t.Error("expected tsx watch with flags to be a server script")
	}
}

// --- Case C: package-manager recursion ---

func TestIsServerScriptPnpmRecurse(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "vite")
	pkg := `{"scripts":{"dev":"pnpm run start-server","start-server":"vite"}}`
	mkFile(t, dir, "package.json", pkg)
	// "dev" recurses into "start-server" which is "vite" → server.
	if !isServerScript(dir, "pnpm run start-server", 0) {
		t.Error("expected 'pnpm run start-server' to be a server script via recursion")
	}
}

func TestIsServerScriptPnpmDirectScriptName(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "vite")
	pkg := `{"scripts":{"dev":"vite"}}`
	mkFile(t, dir, "package.json", pkg)
	// pnpm dev → runs "dev" script which is "vite".
	if !isServerScript(dir, "pnpm dev", 0) {
		t.Error("expected 'pnpm dev' to be a server script via pnpm script recursion")
	}
}

func TestIsServerScriptDepthLimit(t *testing.T) {
	dir := t.TempDir()
	// Circular: a -> pnpm run b, b -> pnpm run a.
	pkg := `{"scripts":{"a":"pnpm run b","b":"pnpm run a"}}`
	mkFile(t, dir, "package.json", pkg)
	// Should terminate without infinite recursion, returning false.
	result := isServerScript(dir, "pnpm run a", 0)
	if result {
		t.Error("circular scripts should not be detected as servers (just return false at depth limit)")
	}
}

// --- Case C with cd: workspace recursion ---

func TestIsServerScriptCdWorkspaceRecurse(t *testing.T) {
	dir := t.TempDir()
	// web/package.json has dev: vite, with vite installed in web.
	mkBin(t, filepath.Join(dir, "web"), "vite")
	mkFile(t, dir, "web/package.json", `{"scripts":{"dev":"vite"}}`)
	// Root script: cd web && pnpm dev
	if !isServerScript(dir, "cd web && pnpm dev", 0) {
		t.Error("expected 'cd web && pnpm dev' to be a server script via workspace recursion")
	}
}

func TestIsServerScriptCdWorkspaceBuildIsNotServer(t *testing.T) {
	dir := t.TempDir()
	// web/package.json has build: vite build — NOT a server.
	mkBin(t, filepath.Join(dir, "web"), "vite")
	mkFile(t, dir, "web/package.json", `{"scripts":{"build":"vite build"}}`)
	if isServerScript(dir, "cd web && pnpm build", 0) {
		t.Error("expected 'cd web && pnpm build' (vite build) NOT to be a server script")
	}
}

// npm run check-env && next dev: second segment is next dev → server.
func TestIsServerScriptCheckEnvAndNextDev(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "next")
	mkFile(t, dir, "package.json", `{"scripts":{"check-env":"node check-env.mjs"}}`)
	if !isServerScript(dir, "npm run check-env && next dev", 0) {
		t.Error("expected 'npm run check-env && next dev' to be a server script")
	}
}

// --- Case D: non-server scripts ---

func TestIsServerScriptTsup(t *testing.T) {
	dir := t.TempDir()
	if isServerScript(dir, "tsup", 0) {
		t.Error("expected 'tsup' NOT to be a server script")
	}
}

func TestIsServerScriptVitest(t *testing.T) {
	dir := t.TempDir()
	if isServerScript(dir, "vitest run", 0) {
		t.Error("expected 'vitest run' NOT to be a server script")
	}
}

func TestIsServerScriptEslint(t *testing.T) {
	dir := t.TempDir()
	if isServerScript(dir, "eslint .", 0) {
		t.Error("expected 'eslint .' NOT to be a server script")
	}
}

func TestIsServerScriptTscBuild(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "tsc")
	if isServerScript(dir, "tsc && vite build", 0) {
		t.Error("expected 'tsc && vite build' NOT to be a server script")
	}
}

func TestIsServerScriptSeedDb(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, dir, "tsx")
	// scripts/seed-db.ts has no server imports.
	mkFile(t, dir, "scripts/seed-db.ts", "console.log('seeding')\n")
	if isServerScript(dir, "tsx scripts/seed-db.ts", 0) {
		t.Error("expected 'tsx scripts/seed-db.ts' NOT to be a server script")
	}
}

// --- Relative import recursion tests (bindsServer) ---

// TestBindsServerDirect: entry file directly imports fastify and calls listen.
func TestBindsServerDirect(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src/server.ts", fastifyEntryContent)
	path := filepath.Join(dir, "src/server.ts")
	if !entryFileBindsServer(path, dir) {
		t.Error("expected direct fastify import+listen to bind server")
	}
}

// TestBindsServerOneHop: entry imports ./impl, impl.ts has fastify + listen.
func TestBindsServerOneHop(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src/index.ts", "import './impl.js'\n")
	mkFile(t, dir, "src/impl.ts", fastifyEntryContent)
	path := filepath.Join(dir, "src/index.ts")
	if !entryFileBindsServer(path, dir) {
		t.Error("expected one-hop relative import to find server signal")
	}
}

// TestBindsServerTwoHops: entry -> cli.ts -> http/server.ts (has fastify).
func TestBindsServerTwoHops(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src/index.ts", "import { registerServe } from './cli/serve.js'\n")
	mkFile(t, dir, "src/cli/serve.ts", "import { makeServer } from '../http/server.js'\n")
	mkFile(t, dir, "src/http/server.ts", fastifyEntryContent)
	path := filepath.Join(dir, "src/index.ts")
	if !entryFileBindsServer(path, dir) {
		t.Error("expected two-hop chain to find server signal")
	}
}

// TestBindsServerCycle: a.ts -> b.ts -> a.ts. Must not loop; return false.
func TestBindsServerCycle(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src/a.ts", "import './b.js'\n")
	mkFile(t, dir, "src/b.ts", "import './a.js'\n")
	path := filepath.Join(dir, "src/a.ts")
	result := entryFileBindsServer(path, dir)
	if result {
		t.Error("cycle with no server signal should return false")
	}
}

// TestBindsServerJsExtensionStripping: import uses .js suffix, disk has .ts.
func TestBindsServerJsExtensionStripping(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src/index.ts", "import './helper.js'\n")
	mkFile(t, dir, "src/helper.ts", fastifyEntryContent)
	path := filepath.Join(dir, "src/index.ts")
	if !entryFileBindsServer(path, dir) {
		t.Error("expected .js-extension import to resolve to .ts file on disk")
	}
}

// TestBindsServerRespectsDepthLimit: chain a->b->c->d->e where only e has
// the signal. maxBindsServerDepth=3 means we can reach at most depth 3 (4
// files: entry=0, b=1, c=2, d=3). e would be depth 4, so it should be
// skipped → false. If we set depth limit higher (by testing directly with
// depth=0 and a visited map) we confirm the chain itself works at depth 4.
func TestBindsServerRespectsDepthLimit(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.ts", "import './b.js'\n")
	mkFile(t, dir, "b.ts", "import './c.js'\n")
	mkFile(t, dir, "c.ts", "import './d.js'\n")
	mkFile(t, dir, "d.ts", "import './e.js'\n")
	mkFile(t, dir, "e.ts", fastifyEntryContent)
	path := filepath.Join(dir, "a.ts")
	// With default depth limit (3), e.ts is at depth 4 → not reached → false.
	if entryFileBindsServer(path, dir) {
		t.Error("expected depth limit to prevent reaching e.ts at depth 4")
	}
	// Direct call starting at b.ts (depth 0) should reach e.ts within limit.
	pathB := filepath.Join(dir, "b.ts")
	if !entryFileBindsServer(pathB, dir) {
		t.Error("expected chain b->c->d->e (depth 3) to find server signal")
	}
}

// --- Integration: Suggest() ---

func TestNodeSuggestsRootScripts(t *testing.T) {
	dir := t.TempDir()
	// Set up: fastify in deps (eligibility gate), tsx binary, entry file with fastify import.
	mkBin(t, dir, "tsx")
	mkFile(t, dir, "src/index.ts", fastifyEntryContent)
	pkg := `{"name":"fmdplanner","dependencies":{"fastify":"4"},"scripts":{"dev":"tsx src/index.ts","build":"tsup"}}`
	mkFile(t, dir, "package.json", pkg)

	got, err := New().Suggest(dir)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	labels := []string{}
	for _, s := range got {
		labels = append(labels, s.Label)
	}
	sort.Strings(labels)
	want := []string{"dev"}
	if len(labels) != len(want) || labels[0] != want[0] {
		t.Fatalf("got %v, want %v", labels, want)
	}
}

func TestNodeSuggestsWorkspaceScripts(t *testing.T) {
	dir := t.TempDir()
	// Root has tsx entry with fastify; web workspace has vite.
	mkBin(t, dir, "tsx")
	mkFile(t, dir, "src/index.ts", fastifyEntryContent)
	root := `{"name":"fmdplanner","dependencies":{"fastify":"4"},"scripts":{"dev":"tsx src/index.ts"}}`
	mkFile(t, dir, "package.json", root)
	mkFile(t, dir, "pnpm-workspace.yaml", "packages:\n  - web\n")
	mkBin(t, filepath.Join(dir, "web"), "vite")
	web := `{"name":"web","dependencies":{"vite":"5"},"scripts":{"dev":"vite"}}`
	mkFile(t, dir, "web/package.json", web)

	got, err := New().Suggest(dir)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	hasRootDev, hasWebDev := false, false
	for _, s := range got {
		if s.Label == "dev" && s.Cmd == "pnpm dev" {
			hasRootDev = true
		}
		if s.Label == "web:dev" && s.Cmd == "pnpm --filter web dev" {
			hasWebDev = true
		}
	}
	if !hasRootDev {
		t.Errorf("missing root dev suggestion: %+v", got)
	}
	if !hasWebDev {
		t.Errorf("missing web:dev suggestion: %+v", got)
	}
}

func TestNodeMissingPackageJsonReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := New().Suggest(dir)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestNodeIneligibleReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	// No server framework dependency → eligibility gate fails → no suggestions
	// even though the script command would otherwise match.
	pkg := `{"name":"qbosync","devDependencies":{"tsx":"4","vitest":"1"},"scripts":{"dev":"tsx src/index.ts"}}`
	mkFile(t, dir, "package.json", pkg)
	got, err := New().Suggest(dir)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty (no server framework dep), got %v", got)
	}
}

func TestNodeReadsNpmStyleWorkspaces(t *testing.T) {
	dir := t.TempDir()
	mkBin(t, filepath.Join(dir, "packages", "ui"), "vite")
	root := `{"name":"r","dependencies":{"vite":"5"},"workspaces":["packages/*"],"scripts":{"dev":"x"}}`
	mkFile(t, dir, "package.json", root)
	mkFile(t, dir, "packages/ui/package.json", `{"name":"ui","scripts":{"dev":"vite"}}`)

	got, err := New().Suggest(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range got {
		if s.Label == "ui:dev" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ui:dev from workspaces glob, got %+v", got)
	}
}

func TestNodeReadsNpmStyleWorkspacesMapForm(t *testing.T) {
	dir := t.TempDir()
	rootPkg := `{
		"name": "monorepo",
		"dependencies": {"fastify": "^4.0.0"},
		"workspaces": {"packages": ["web"]}
	}`
	mkFile(t, dir, "package.json", rootPkg)
	mkFile(t, dir, "pnpm-lock.yaml", "lockfileVersion: 6\n")
	mkBin(t, filepath.Join(dir, "web"), "vite")
	webPkg := `{"name": "web", "dependencies": {"vite": "^5.0.0"}, "scripts": {"dev": "vite"}}`
	mkFile(t, dir, "web/package.json", webPkg)

	p := New()
	sugg, err := p.Suggest(dir)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(sugg) != 1 || sugg[0].Cmd != "pnpm --filter web dev" {
		t.Errorf("got %+v, want one suggestion with Cmd 'pnpm --filter web dev'", sugg)
	}
}

func TestDetectPackageManagerYarn(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "yarn.lock", "")
	if got := detectPackageManager(dir); got != "yarn" {
		t.Errorf("got %q, want yarn", got)
	}
}

func TestDetectPackageManagerNpm(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "package-lock.json", "{}")
	if got := detectPackageManager(dir); got != "npm" {
		t.Errorf("got %q, want npm", got)
	}
}

func TestDetectPackageManagerBun(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "bun.lockb", "")
	if got := detectPackageManager(dir); got != "bun" {
		t.Errorf("got %q, want bun", got)
	}
}

func TestDetectPackageManagerExplicitField(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "package.json", `{"packageManager":"yarn@4.0.0"}`)
	mkFile(t, dir, "pnpm-lock.yaml", "") // should be overridden by explicit field
	if got := detectPackageManager(dir); got != "yarn" {
		t.Errorf("got %q, want yarn (explicit packageManager wins)", got)
	}
}

func TestDetectPackageManagerFallback(t *testing.T) {
	dir := t.TempDir()
	if got := detectPackageManager(dir); got != "npm" {
		t.Errorf("got %q, want npm (fallback)", got)
	}
}

func TestNodeRejectsWorkspacePathEscape(t *testing.T) {
	// Create an "escape" directory outside the project root with a package.json
	// that has a server script. The plugin must not walk into it.
	outer := t.TempDir()
	escapeDir := filepath.Join(outer, "escape")
	mkBin(t, escapeDir, "vite")
	escapePkg := `{"name":"escape","dependencies":{"vite":"5"},"scripts":{"dev":"vite"}}`
	if err := os.WriteFile(filepath.Join(escapeDir, "package.json"), []byte(escapePkg), 0644); err != nil {
		t.Fatal(err)
	}

	// Project lives one level deeper; pnpm-workspace.yaml points at ../escape.
	projectDir := filepath.Join(outer, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	rootPkg := `{"name":"root","dependencies":{"fastify":"4"},"scripts":{}}`
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}
	wsYaml := "packages:\n  - ../escape\n"
	if err := os.WriteFile(filepath.Join(projectDir, "pnpm-workspace.yaml"), []byte(wsYaml), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := New().Suggest(projectDir)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	for _, s := range got {
		if s.Label == "escape:dev" {
			t.Errorf("path-escape workspace should be silently dropped, got suggestion: %+v", s)
		}
	}
}
