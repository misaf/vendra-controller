# Running the stack from local source

The controller normally pulls digest-pinned images from a registry. For local
work you can build them yourself and run the stack with no registry at all.

Two things make that possible:

- `--no-pull` skips `docker compose pull`. Without it the run fails immediately:
  Compose exits `1` on an image that is not in a registry, even though
  `docker compose up` would have started it happily from the local daemon.
- `images.website` is optional. With it empty the website project is not
  rendered or started, and the apex host stays free.

## 1. Build the images you have

```sh
# Provisioner (this repository)
docker build -t vendra-controller:local .

# Storefront
cd ../vendra-storefront-florist && docker build -t vendra-storefront-florist:local .

# Platform (Laravel application)
cd ../vendra-platform && docker build -f docker/Dockerfile -t vendra-platform:local .
```

## 2. Point the controller at them

```yaml
# controller.local.yaml
state_dir: /tmp/vendra-local
base_domain: vendra.test
network: traefik-public
certificate_mode: self-signed      # no ACME, and HSTS stays off
images:
  platform: vendra-platform:local
  website: ""                      # omitted: no local source for it
  storefront: vendra-storefront-florist:local
  provisioner: vendra-controller:local
no_pull: true                      # see below
```

`no_pull: true` is the config form of `--no-pull`, and it is the only one the
provisioner obeys. Deploying a property from the console does not go through the
CLI: the console calls the provisioner, which runs as a container in the platform
stack and has no flags. Without this key it pulls the storefront image and the
deployment lands in `storefront_deployments` as `failed` with a bare
`provisioning failed` — the registry error is only in `docker logs
vendra-provisioner-1`. Prefer `vendra stack logs provisioner`, which does not
depend on the generated container name.

Tags rather than digests are correct here — a digest pins an image that only
exists in a registry. Production still pins by digest.

**The names must match what you built exactly.** A registry-shaped name such as
`ghcr.io/misaf/vendra:local` is not the same image as `vendra-platform:local`,
even with the same contents. Check with `docker images` before starting; a
mismatch reads as *No such image*.

## 3. Start it

```sh
vendra init  --config ./controller.local.yaml
vendra stack up --no-pull --config ./controller.local.yaml
sudo vendra stack hosts --write --config ./controller.local.yaml
```

## 4. Add a storefront property

The configuration must satisfy the storefront's schema, not just slug and
domain. Generate one from a property directory in the storefront repository:

```sh
cd ../vendra-storefront-florist
npm run property:config <slug> -- --json > /tmp/<slug>.json

vendra property render <slug> <slug>.vendra.test \
  --image vendra-storefront-florist:local \
  --configuration /tmp/<slug>.json \
  --config ./controller.local.yaml

vendra property up <slug> --no-pull --config ./controller.local.yaml
```

Then browse `https://<slug>.vendra.test/en`. The certificate is issued by a
local authority, so the browser will warn; HSTS is deliberately disabled in this
mode, so the "proceed anyway" bypass still works. Step 5 is not optional — the
page will look broken until you have accepted the API origin too.

## 5. Accept the storefront certificate

A browser uses only the storefront origin for normal reads. Client GET requests
go through the same-origin `/api/proxy/*` route; the storefront container calls
the canonical API and trusts the generated authority through
`NODE_EXTRA_CA_CERTS`.

Open `https://<slug>.<base_domain>` and accept that certificate warning.

If proxied requests fail with a certificate error, inspect the property
container instead of accepting the API certificate in the browser. It must mount
the controller's `certificates/` directory and receive
`NODE_EXTRA_CA_CERTS=/certs/ca.pem`.

This needs no administrator rights — a browser exception is stored in your own
profile. Chrome sometimes needs a hard reload afterwards before it retries the
requests that already failed.

The exception survives re-renders because certificates are re-signed from a
persistent authority (`certificates/ca.pem`). Only deleting `ca.pem` (or
`state_dir`) invalidates it. Browser exceptions are per profile and hostname, so
each new property hostname must be accepted separately.

### Optional: trust the authority outright

With administrator rights you can install the authority instead and see no
warnings at all. This is a convenience, not a prerequisite — it is the same
certificate either way:

```sh
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain \
  /var/lib/vendra/certificates/ca.pem
```

Then quit and reopen the browser — Chrome and Safari cache TLS decisions for the
life of the process.

## What to expect

- Storefront containers report healthy via `/api/health`.
- Routers carry `www-redirect`, `security-headers` and `compression`, so
  responses include `X-Frame-Options`, `X-Content-Type-Options` and
  `Referrer-Policy`.
- **No** `Strict-Transport-Security` header. That is correct under
  `certificate_mode: self-signed` — see `security.md`.
- The Traefik dashboard is on `http://127.0.0.1:8080/dashboard/`.

## Caveats

`--no-pull` skips the explicit `compose pull` only; it does not pass
`--pull never` to `compose up`. That is deliberate — the stack's public
dependencies (mysql, redis, traefik) are legitimately fetched from a registry,
and forbidding that breaks a first run on a clean machine. `up` uses a locally
built image when one is present under the configured name.

A stale local tag is used silently: rebuild the image and run
`vendra property restart <slug>` after changing source. Nothing checks that the
tag matches what you last built.

With `images.website` empty, nothing serves the apex `BASE_DOMAIN` host. Only
property and platform hosts respond.
