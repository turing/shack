# shack 🛖

Point shack at a repo and your dev servers come up on pretty, trusted `https://*.localhost` URLs. It discovers which of your scripts bind which ports — so there's no Caddyfile to edit and no ports to wire up by hand.

```sh
cd myapp/
shack init
shack up
```

macOS resolves every `*.localhost` to `127.0.0.1` natively — no `/etc/hosts`, no DNS. shack drives Caddy + tmux and handles the rest. Ultra lightweight little groups of services are easy to manage.

> [!IMPORTANT]
> **macOS + Homebrew only.**

## Install

```sh
brew install go caddy tmux           # build + runtime deps
git clone https://github.com/turing/shack
cd shack
make install                         # → $(brew --prefix)/bin/shack
sudo caddy trust                     # one-time, see below
```

`sudo caddy trust` installs Caddy's local certificate authority into your macOS keychain so browsers accept `*.localhost` certs without a warning. Skip it and every `https://*.localhost` shows "Your connection is not private." Confirm everything with `shack status`.

## Set up a project

This is the main way to use shack. Run the wizard once in a repo:

```sh
cd ~/code/myapp
shack init
```

`init` walks your `package.json` (pnpm / yarn / npm / bun), finds the scripts that start a server, lets you pick which ones to manage, assigns each a hostname (`myapp-server.localhost`, …), and chooses which boot together. It writes `.shack/config.json`. Then:

```sh
shack up           # boots caddy + every default service in tmux, and shows
                   # them come up live — each collapses to "✓ https://…" the
                   # moment its URL actually responds
shack dev:server   # run one service directly; its URL registers while it's
                   # alive and unregisters when it exits
shack attach       # jump into the tmux session (shack attach dev:server for one)
shack down         # tear this project's session down
```

`shack up` streams each service's boot output, probes its real URL through caddy, and collapses to a tidy summary once everything serves — or leaves the logs on screen for whatever didn't. Detach from tmux with `Ctrl-b` then `d`; your servers keep running. `shack <label>` is transparent: colors, hot-reload, prompts, `Ctrl-C`, and terminal resize all pass straight through.

## Ad-hoc proxy

For a one-off service you just want on a hostname (no project, no config):

```sh
shack add myapp 3000     # register myapp.localhost → :3000, reload caddy
shack list               # every registration + alive/dead state
shack rm myapp           # remove one
shack drop               # drop registrations whose port went silent
```

## Config

`shack init` writes `.shack/config.json`. Hand-edit values you trust:

```json
{
  "name": "myapp",
  "defaults": ["dev:web", "dev:server"],
  "commands": [
    {
      "label": "dev:server",
      "cmd": "pnpm dev:server",
      "pre": "pnpm migrate",
      "listeners": [{ "match": { "script": "dev:server" }, "as": "myapp-server" }]
    }
  ]
}
```

- `match` is one of `script` / `package` / `comm`, matched against `npm_lifecycle_event`, `npm_package_name`, or the process name of each new listener.
- `pre` (optional) runs before the command and must exit 0.
- Ports are never stored — they're discovered at runtime via `lsof`.
- `defaults` is the set `shack up` launches together.

## Commands

| Command | Does |
|---|---|
| `init` | discover a project's servers, write `.shack/config.json` |
| `up [all]` | boot the project's default services in tmux |
| `<label>` | run a configured command, registering its URL while alive |
| `attach [service]` | attach to the session, optionally a service's window |
| `down [all]` | tear down this project's session (`all`: every session + caddy) |
| `add <name> <port>` | register `<name>.localhost → :<port>` |
| `rm <name>` | remove a registration |
| `list` | registrations + alive/dead |
| `drop` | drop registrations whose port is gone |
| `status` | caddy state, Caddyfile path, CA trust |
| `reload` | reload caddy (rarely needed by hand) |

## How it works

- The Caddyfile is the only persistent state — shack owns a delimited region inside `$(brew --prefix)/etc/Caddyfile` and never touches anything outside it.
- No daemon, no database, no HTTP API. Each command is a one-shot run that edits the Caddyfile and reloads caddy.
- Ports are discovered, never recorded — shack walks the process tree and snapshots TCP listeners with `lsof`.
- `shack up` runs each service as its own tmux pane and probes its `https://<host>.localhost` URL through caddy (using your system's trust) to know when it's genuinely up.

## Develop

```sh
make test               # unit tests
make test-integration   # end-to-end against real brew + caddy
                        # (mutates the Caddyfile's managed region, then cleans up)
make build              # → ./shack
make vet                # go vet
```

The Node plugin in `internal/plugins/` is the template for adding other runtimes. Contributions welcome.

## License

MIT.
