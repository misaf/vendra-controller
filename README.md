# Vendra Controller

`vendra-controller` is Vendra's Go infrastructure control plane. It ships two
binaries built from the same internal packages:

- `vendra`: the host CLI for stack and property lifecycle management.
- `provisioner`: the authenticated internal HTTP service used by Laravel.

Production hosts require only Docker with Compose v2 and the prebuilt `vendra`
binary. PHP, Composer, Python, source checkouts, and shell scripts are not host
dependencies.

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

