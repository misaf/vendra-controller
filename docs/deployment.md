# Deployment and CLI

## Prerequisites

A deployment host needs Docker with Compose v2 and the `vendra` executable.
The executable is not committed to this repository. During development, build
it from the repository root with:

```sh
mkdir -p bin
go build -o ./bin/vendra ./cmd/vendra
```

Use `./bin/vendra` from the checkout, or install it as
`/usr/local/bin/vendra`. Production deployments should use a versioned release
artifact built for the host platform rather than requiring Go or a source
checkout on the server.

Confirm both dependencies before deployment:

```sh
vendra version
docker compose version
```

An error such as `command not found: vendra` means the binary has not been
built/installed or its directory is not in `PATH`; it is not created by
`vendra init` or by Docker Compose.

Configuration precedence is flags, process environment, controller environment
file, YAML config, then defaults. Run `vendra completion <shell>` for completion.

`vendra stack up` renders embedded manifests, creates `traefik-public`, generates
local certificates when configured, pulls pinned images, validates Compose, and
waits for services. `down`, `restart`, `ps`, `logs`, `urls`, `hosts`, and
`status` operate on the same state directory.

Property commands are `add`, `render`, `up`, `down`, `restart`, and `remove`.
`add` refuses existing state; `render` is idempotent and preserves unknown `.env`
keys while replacing the controller-owned Compose manifest.
