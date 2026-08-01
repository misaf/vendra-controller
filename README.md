# Vendra Controller

`vendra-controller` is Vendra's Go infrastructure control plane. It ships two
binaries built from the same internal packages:

- `vendra`: the host CLI for stack and property lifecycle management.
- `provisioner`: the authenticated internal HTTP service used by Laravel.

Production hosts require only Docker with Compose v2 and the prebuilt `vendra`
binary. PHP, Composer, Python, source checkouts, and shell scripts are not host
dependencies.

## Installing the CLI

The repository does not contain a compiled `vendra` binary. Developers can
build it from the repository root:

```sh
mkdir -p bin
go build -o ./bin/vendra ./cmd/vendra
./bin/vendra version
```

This creates `bin/vendra` in the current checkout. To make it available on the
system `PATH`:

```sh
sudo install -m 0755 ./bin/vendra /usr/local/bin/vendra
vendra version
```

Production hosts should install a versioned release artifact instead of
building from a source checkout. The release artifact must match the host's
operating system and CPU architecture. Verify the installed binary with
`vendra version` before initializing the stack.

Pushing a semantic-version tag such as `v1.0.0` builds and publishes the
provisioner container as `ghcr.io/<owner>/vendra-controller`. The workflow
publishes the full version, major/minor, and `latest` tags. This image package
is separate from the host `vendra` CLI binary.

If the shell reports `command not found: vendra`, either invoke the development
binary as `./bin/vendra` or install it into a directory included in `PATH`.

## Start here

For a complete installation—from a new host to the first storefront—follow
[Getting started](docs/getting-started.md). The abbreviated command sequence
below assumes Docker, the CLI, DNS, configuration, secrets, and registry access
are already prepared.

## Quick reference

```text
vendra init --config /etc/vendra/controller.yaml
vendra stack up
vendra stack status
vendra property render acme acme.example.com --configuration config.json
```

Copy `controller.example.yaml` to `/etc/vendra/controller.yaml`, pin every image
by digest, and place secrets such as `VENDRA_PROVISIONER_TOKEN`, `APP_KEY`, and
database credentials in `/etc/vendra/controller.env` with mode `0600`.

See [the architecture](docs/architecture.md), [deployment](docs/deployment.md),
[provisioning](docs/provisioning.md), [security](docs/security.md), and
[recovery](docs/recovery.md) documentation.
