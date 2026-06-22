# shack

Stop hand-editing your Caddyfile every time you spin up a dev server.
Drop a service onto `https://myapp.localhost` with one command, with a
trusted certificate, and let shack tear it down when you're done.
For repeated workflows, wrap your dev commands directly:

```sh
shack add myapp 3000          # myapp.localhost → localhost:3000
shack dev:server              # wraps `pnpm dev:server`, registers
                                   # myapp-server.localhost while it runs
```

macOS resolves any `*.localhost` to `127.0.0.1` natively — no `/etc/hosts`,
no DNS, no setup. shack just makes the rest work.

macOS + Homebrew only.

## Install

```sh
brew install go caddy tmux       # all three are required
make install                     # → /opt/homebrew/bin/shack (Apple
                                 # Silicon) or /usr/local/bin/shack
                                 # (Intel)
sudo caddy trust                 # one-time, see below
```

`make install` checks for `go`, `caddy`, and `tmux` first and fails fast
with an install hint if any is missing.

`sudo caddy trust` is a one-time step. Caddy generates its own local
certificate authority to sign certificates for `*.localhost`. `caddy trust`
registers that CA in the macOS system keychain so browsers accept those
certificates without a warning. Without it, every visit to
`https://myapp.localhost` shows "Your connection is not private."

Verify the install:

```sh
shack status
# caddy: stopped|running
# caddyfile: /opt/homebrew/etc/Caddyfile
# entries: 0
# ca: trusted|not trusted (run: sudo caddy trust)
```

## Quick start: ad-hoc proxy

You have a service running on port 3000. You want it at
`https://myapp.localhost`:

```sh
shack add myapp 3000     # ensures caddy is running, registers myapp.localhost,
                              # caddy reload — done. Browser to https://myapp.localhost.
shack list               # show all registrations + alive/dead state
shack rm myapp           # remove
shack gc                 # drop entries whose ports have nothing listening
shack status             # caddy state + caddyfile path + CA trust + entry count
shack reload             # caddy reload (rarely needed manually)
```

`alive` means something is currently listening on the registered port.
`dead` means the registration is stale — the service that was there is gone.
`gc` sweeps dead entries; mutating commands (`add`, `rm`) sweep them as a
side effect.

## Per-project setup: full workflow

For repos you develop regularly, run `shack init` once:

```sh
cd ~/Repositories/myapp
shack init
```

`init` is an interactive TUI wizard. The full flow:

1. **Overwrite check** — if `.shack/config.json` already exists, `init`
   prompts to overwrite. `n` exits without changes.
2. **Project name** — defaults to the directory name; must match
   `^[a-z0-9][a-z0-9-]*$`. Loops on invalid input.
3. **Plugin discovery** — silent; the Node plugin walks `package.json` and
   recursively traces relative imports up to depth 3 to find scripts that
   bind a server.
4. **Test selection** — multi-select of detected scripts, defaulting to
   nothing checked. Free-text area below for custom commands the plugin
   missed (e.g. `caddy run --config Caddyfile`).
5. **Test runs** — shack briefly executes each selected command in
   its own process group, watches `lsof` for new TCP listeners, then
   SIGINTs it. Output is captured to a buffer; if a test fails, the
   buffer is dumped after the spinner so you can see what happened.
6. **Unbound-script recovery** — for each selected script that didn't
   bind a port, you can choose: configure as-is (use the original
   command), rewrite the command (e.g. `pnpm dev` is broken; `pnpm dev
   serve` is what you want), or skip.
7. **Hostname assignment** — per detected listener, defaulting to
   `<project>-<script-suffix>` (e.g. `myapp-server`). Validates against
   the name regex.
8. **Collision rename** — if any label matches a reserved subcommand
   (`add`, `gc`, `init`, `list`, `reload`, `rm`, `up`, `status`, `down`,
   `attach`), `init` forces a rename to free the namespace.
9. **Pre-run hook** — optional `pre` command per binder (e.g.
   `pnpm migrate` before `pnpm dev:server`). Blank skips.
10. **Default group selection** — multi-select of which commands run
    together when you type `shack up`. Has an explicit "(none)"
    option.
11. **Confirm and write** — final summary, then writes
    `.shack/config.json`.

After init, two ways to run:

```sh
shack dev:server         # single command — runs your script directly,
                              # registers myapp-server.localhost while alive,
                              # unregisters on exit
shack up                 # tmux group — boots caddy, opens a tmux session
                              # with one window per default-group command
shack attach             # attach to the tmux session (or switch to it
                              # if you're already in tmux)
shack down               # kill the current project's tmux session
shack down all           # kill EVERY shack-* tmux session +
                              # stop the caddy daemon
```

`shack <label>` is transparent: stdout, stderr, ANSI colours, hot-reload
output, interactive prompts, terminal resize signals, Ctrl-C all work
exactly as if you ran the wrapped command directly. (Naïve wrappers tend
to break: stdout buffering kills colour, lost SIGWINCH breaks resize, lost
SIGINT leaves orphans. shack uses `tcsetpgrp` to give the child a
real PTY and forwards signals correctly.)

`shack up` requires `tmux` and uses one window per service. It
returns immediately after spawning the session — shack itself is
not the parent of your dev servers; tmux owns their lifetime.

> **Note on `shack down` outside a project:** if you run `down` in a
> directory that isn't a shack project, it stops the caddy daemon
> entirely. While caddy is down, every `*.localhost` registration on your
> machine stops responding. The Caddyfile entries are preserved — they
> come back online when caddy restarts. To scope a kill to one project's
> tmux session, run `down` from inside that project.

> **Note on the 60 s drop-alias:** if `shack <label>` runs for 60 s
> without observing a port bind that matches the configured listener,
> shack treats the config as stale, rewrites
> `.shack/config.json` to remove that command, and prints a warning
> when the command exits. Re-run `shack init` to restore it. The
> child process is not killed.

## Config schema

`.shack/config.json`:

```json
{
  "name": "myapp",
  "defaults": ["dev:web", "dev:server"],
  "commands": [
    {
      "label": "dev:server",
      "cmd": "pnpm dev:server",
      "pre": "pnpm migrate",
      "listeners": [
        { "match": { "script": "dev:server" }, "as": "myapp-server" }
      ]
    },
    {
      "label": "dev:web",
      "cmd": "pnpm dev:web",
      "listeners": [
        { "match": { "script": "dev" }, "as": "myapp-web" }
      ]
    }
  ]
}
```

- `match` has exactly one of `script` / `package` / `comm`. At runtime,
  shack reads `npm_lifecycle_event`, `npm_package_name`, and the
  process basename from each newly observed listener and picks the first
  matching listener entry. To hand-edit: `script` should be the value
  of `npm_lifecycle_event` you expect at runtime (usually the script
  name from `package.json`); `package` is `npm_package_name` (workspace
  name); `comm` is the process basename (e.g. `node`, `vite`).
- `pre` is optional. If present, runs synchronously (`sh -c`) before
  the main command. Must exit 0 or the command doesn't run.
- Ports are never stored. They're discovered at runtime via `lsof`.
- `defaults` lists labels that `shack up` runs together as a group.

`init` is the canonical way to produce or update this file. Hand-edit
field values you trust (hostnames, pre hooks). Re-run `init` if you've
added scripts or changed listener layout.

## Plugins

Currently ships a Node plugin that auto-detects server scripts in
`package.json` (supports pnpm/yarn/npm/bun via lockfile + `packageManager`
field detection; deno entry files are recognized in introspection but
not in package-manager command generation). Easily expandable to other
runtimes — see `internal/plugins/`.

## How it works

shack is a proxy setup with some nice conveniences. Mechanics:

- **Caddyfile is the only persistent shared state.** shack owns a
  delimited region inside `$(brew --prefix)/etc/Caddyfile`; everything
  outside the region is preserved across writes.
- **No daemon, no database, no HTTP API.** Each `shack` invocation
  is a one-shot CLI run that mutates the Caddyfile and asks caddy to
  reload.
- **Port discovery via `lsof`.** Ports aren't recorded anywhere — they're
  observed at runtime by walking the process tree (`pgrep -P`) and
  snapshotting TCP listeners (`lsof -nP -iTCP -sTCP:LISTEN -p <pids>`).
- **`shack <label>`** spawns the wrapped command in its own process
  group, makes it the foreground PG via `tcsetpgrp` (so signals and
  terminal modes route correctly), forwards SIGINT/SIGTERM/SIGHUP, and
  on exit does a single Caddyfile rewrite + reload to unregister
  everything atomically.
- **`shack up`** shells out to `tmux new-session -d` with one window
  per default-group label.

For deeper internals (drop-alias logic, plugin architecture, signal
handling edge cases) see `docs/specs/` and `docs/plans/`.

## Preflight

Most ops require `brew`, `caddy`, and a trusted CA. Specifically:

- **`add`, `rm`, `list`, `gc`, `reload`** — require `brew` on PATH,
  `caddy` installed, and CA trusted.
- **`init`, `up`, `down`, `attach`** — additionally require `tmux` on
  PATH (init writes a config that `up`/`down`/`attach` will need; the
  others use tmux directly).
- **`status`** — only requires `brew` and `caddy`. CA-trust failure is
  reported in the output rather than aborting the command, so you can
  always diagnose.

## Development

```sh
make test                         # unit tests
make test-integration             # end-to-end against real brew + caddy.
                                  # Modifies your real /opt/homebrew/etc/Caddyfile
                                  # via the managed-region mechanism. Cleans up
                                  # after itself but assumes you have
                                  # shack's preflight stack already
                                  # installed (brew + caddy + tmux).
make build                        # produces ./shack
```

## License

MIT.
