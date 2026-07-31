# Security Model

Only the provisioner container receives the write-capable Docker socket. Laravel
web, API, Horizon, scheduler, and monitoring containers never receive it.
Traefik uses a separate endpoint-restricted read-only socket proxy.

Provisioner v1 routes use constant-time bearer authentication. Tokens are read
from environment-only secret configuration and structured logs never include
request payloads, tokens, or generated environment values.

