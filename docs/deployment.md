# Deployment and CLI

Configuration precedence is flags, process environment, controller environment
file, YAML config, then defaults. Run `vendra completion <shell>` for completion.

`vendra stack up` renders embedded manifests, creates `traefik-public`, generates
local certificates when configured, pulls pinned images, validates Compose, and
waits for services. `down`, `restart`, `ps`, `logs`, `urls`, `hosts`, and
`status` operate on the same state directory.

Property commands are `add`, `render`, `up`, `down`, `restart`, and `remove`.
`add` refuses existing state; `render` is idempotent and preserves unknown `.env`
keys while replacing the controller-owned Compose manifest.

