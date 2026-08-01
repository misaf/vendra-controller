# Getting started from scratch

This guide takes a new Vendra host from an empty server to a running platform
and its first storefront. Commands are written for a Linux production host.
Use the development notes where your workstation differs.

## 1. Understand what gets installed

The `vendra` CLI runs on the host and manages four kinds of Docker Compose
projects:

- `proxy`: Traefik and its restricted Docker socket proxy.
- `platform`: MySQL, Redis, Laravel, Horizon, the scheduler, and provisioner.
- `website`: the public Vendra website.
- one project per storefront property.

Generated runtime files live under `/var/lib/vendra` by default. Do not edit
their controller-owned Compose files manually; the next render replaces them.

## 2. Prepare the host

Use a dedicated Linux host with:

- Docker Engine and Docker Compose v2 (`docker compose`, not the legacy
  `docker-compose` command).
- TCP ports 80 and 443 reachable from the internet for a public deployment.
- Enough disk space for application images, MySQL data, and storefronts.
- Access to the container registry containing the four Vendra images.

Verify Docker before continuing:

```sh
docker info
docker compose version
```

If the registry is private, authenticate using credentials authorized to pull
the images:

```sh
docker login ghcr.io
```

## 3. Obtain the `vendra` CLI

The source repository does not contain a compiled executable.

For development, install the Go version declared in `go.mod`, clone the
repository, and run from its root:

```sh
mkdir -p bin
go build -o ./bin/vendra ./cmd/vendra
./bin/vendra version
```

Optionally put the development build on `PATH`:

```sh
sudo install -m 0755 ./bin/vendra /usr/local/bin/vendra
vendra version
```

For production, download the versioned release artifact for the host operating
system and CPU architecture, verify its published checksum, and install it as
`/usr/local/bin/vendra`. Do not install Go or clone the source repository on a
production host merely to run the controller.

> The release-image workflow publishes the provisioner container, but it does
> not currently produce downloadable `vendra` CLI binaries. Until a CLI release
> workflow is added, an authorized build machine must build and distribute the
> host executable.

## 4. Configure DNS

Choose the base domain, for example `example.com`. Point these names to the
host's public IP address:

```text
example.com
www.example.com
api.example.com
admin.example.com
console.example.com
reseller.example.com
```

Each custom storefront domain, including its `www` name when used, must also
point to the host. Wait for DNS to resolve before requesting public TLS
certificates.

For local development, use `vendra.test` and the self-signed certificate mode;
the CLI can generate the required hosts-file entry later.

## 5. Create the controller configuration

Create the configuration directory:

```sh
sudo mkdir -p /etc/vendra
```

Create `/etc/vendra/controller.yaml`:

```yaml
state_dir: /var/lib/vendra
base_domain: example.com
network: traefik-public
listen: :8080
health_timeout: 2m
certificate_mode: acme
acme_email: ops@example.com
images:
  platform: ghcr.io/your-org/vendra@sha256:PLATFORM_DIGEST
  website: ghcr.io/your-org/vendra-website@sha256:WEBSITE_DIGEST
  storefront: ghcr.io/your-org/vendra-storefront@sha256:STOREFRONT_DIGEST
  provisioner: ghcr.io/your-org/vendra-controller@sha256:PROVISIONER_DIGEST
```

Replace every example value. Production image references should be immutable
digest-pinned references, not mutable tags such as `latest`.

`images.storefront` is a fallback image, not a list of all storefront images.
It is used only when a manual `vendra property add` or `property render` command
does not provide `--image`. Each property stores its selected image in its own
generated `.env` file, so different properties can run different images at the
same time.

For a local installation, change these values:

```yaml
state_dir: /absolute/writable/path/to/vendra-state
base_domain: vendra.test
certificate_mode: self-signed
acme_email: ""
```

## 6. Create secrets

Generate independent random database and provisioner credentials. Generate a
Laravel application key in the format expected by the platform image. Then
create `/etc/vendra/controller.env`:

```dotenv
VENDRA_PROVISIONER_TOKEN=replace-with-a-long-random-token
APP_KEY=base64:replace-with-a-laravel-application-key
DB_DATABASE=vendra
DB_USERNAME=vendra
DB_PASSWORD=replace-with-a-database-password
DB_ROOT_PASSWORD=replace-with-a-different-root-password
MAIL_MAILER=smtp
MAIL_HOST=smtp.example.com
MAIL_PORT=587
```

Protect the file:

```sh
sudo chown root:root /etc/vendra/controller.env
sudo chmod 0600 /etc/vendra/controller.env
```

The controller reads this exact environment-file path automatically. Existing
process environment variables take precedence over values in the file.

Do not commit this file, paste it into tickets, or place secrets in
`controller.yaml`.

## 7. Initialize and start the stack

Run:

```sh
sudo vendra init --config /etc/vendra/controller.yaml
sudo vendra stack up --config /etc/vendra/controller.yaml
```

`init` creates the state directories and renders the Compose projects. `stack
up` checks Docker, creates the shared network, renders again, prepares
certificates, validates Compose, pulls images, starts the projects, and waits
for their health checks.

Re-running `stack up` is the normal convergent operation after configuration or
image changes.

## 8. Verify the deployment

Check container state and print the expected URLs:

```sh
sudo vendra stack status --config /etc/vendra/controller.yaml
sudo vendra stack urls --config /etc/vendra/controller.yaml
```

The URL list includes the website, API health endpoint, admin, console,
reseller, and Traefik dashboard hostname. Confirm the services you intend to
expose respond over HTTPS.

Inspect logs when a service is unhealthy:

```sh
sudo vendra stack logs php --config /etc/vendra/controller.yaml
sudo vendra stack logs horizon --config /etc/vendra/controller.yaml
sudo vendra stack logs website --config /etc/vendra/controller.yaml
sudo vendra stack logs proxy --config /etc/vendra/controller.yaml
```

Log commands follow output continuously; press Ctrl-C to stop following them.

For local development, print or install the hosts-file entries:

```sh
./bin/vendra stack hosts --config ./controller.local.yaml
sudo ./bin/vendra stack hosts --write --config ./controller.local.yaml
```

Trusting the generated local certificate is an operating-system/browser step
and is not performed by the controller.

## 9. Create the first storefront

In normal operation Laravel sends an authenticated request to the internal
provisioner. For a manual smoke test, create `storefront.json` whose identity
matches the CLI arguments:

```json
{
  "slug": "acme",
  "domain": "shop.example.com"
}
```

Render and start it:

```sh
sudo vendra property add acme shop.example.com \
  --image ghcr.io/your-org/storefront-florist@sha256:FLORIST_DIGEST \
  --configuration ./storefront.json \
  --config /etc/vendra/controller.yaml
sudo vendra property up acme --config /etc/vendra/controller.yaml
sudo vendra stack logs acme --config /etc/vendra/controller.yaml
```

Select the appropriate image for every property. For example, another property
can use a separately built image without changing the controller configuration:

```sh
sudo vendra property add bakery shop.bakery.example \
  --image ghcr.io/your-org/storefront-bakery@sha256:BAKERY_DIGEST \
  --configuration ./bakery.json
sudo vendra property up bakery
```

Laravel also sends an `image` in every provisioner request, and that value is
used for that property. The current provisioner accepts only the
`vendra-storefront-florist` template name, although the image itself is selected
per request. Supporting several template names requires extending the
provisioner's template validation; it is separate from supporting several image
versions or repositories.

The slug may contain lowercase letters, digits, and internal hyphens. The
configuration must be valid JSON and its `slug` and `domain` must exactly match
the command arguments.

To update controller-managed property configuration, render it again and
restart the property:

```sh
sudo vendra property render acme shop.example.com \
  --image ghcr.io/your-org/storefront-florist@sha256:NEW_DIGEST \
  --configuration ./storefront.json \
  --config /etc/vendra/controller.yaml
sudo vendra property restart acme --config /etc/vendra/controller.yaml
```

## 10. Routine operations

```sh
# Show all Compose projects
sudo vendra stack ps

# Restart the complete stack
sudo vendra stack restart

# Stop the complete stack
sudo vendra stack down

# Stop or start one storefront
sudo vendra property down acme
sudo vendra property up acme

# Permanently remove one storefront's generated runtime state
sudo vendra property remove acme
```

Commands use `/etc/vendra/controller.yaml` by default, so `--config` is
optional when that path is used.

## 11. Upgrade safely

Before an upgrade, back up the database, `/etc/vendra`, and
`/var/lib/vendra/acme`. Then:

1. Install the desired `vendra` CLI release.
2. Replace image digests in `controller.yaml` with the matching tested set.
3. Run `vendra stack up`.
4. Run `vendra stack status` and check application logs.

Do not update only one tightly coupled platform component unless that version
combination has been tested.

## 12. Troubleshooting

`command not found: vendra`
: Build the development binary and run `./bin/vendra`, or install the release
  binary into a directory in `PATH`.

`docker unavailable`
: Ensure Docker is running and the current user (or `sudo`) can access the
  Docker daemon.

Compose reports an image variable as required
: Replace all image placeholders in `controller.yaml`. Confirm the selected
  config path and any `VENDRA_*_IMAGE` environment overrides.

Compose reports a database password as required
: Add `DB_PASSWORD` and `DB_ROOT_PASSWORD` to
  `/etc/vendra/controller.env`, then run `vendra stack up` again.

Image pull is denied
: Run `docker login` for the correct registry and verify that the image and
  digest exist and the account has pull permission.

HTTPS or ACME fails
: Verify public DNS, inbound ports 80/443, the ACME email, and proxy logs.
  Local/private hosts should use `certificate_mode: self-signed` instead.

A service does not become healthy
: Run `vendra stack status`, then `vendra stack logs <target>`. Common causes
  are invalid application secrets, database startup failures, incorrect image
  architecture, or an image whose health endpoint does not match the embedded
  Compose definition.

A storefront configuration is rejected
: Verify that it is valid JSON and that its `slug` and `domain` fields exactly
  match the CLI arguments.
