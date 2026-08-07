# Security model

**This document now lives in the ecosystem documentation:**
<https://misaf.github.io/vendra-ecosystem-docs/controller/security>

It covers the Docker socket boundary, published ports and SSH tunnels, the two
header middlewares, why HSTS depends on `certificate_mode`, and what to back up.

The middleware definitions themselves are in
`assets/compose/proxy/dynamic/middlewares.yml`.
