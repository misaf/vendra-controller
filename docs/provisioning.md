# Provisioning

```mermaid
sequenceDiagram
    participant L as Laravel job
    participant P as Go provisioner
    participant D as Docker
    L->>P: POST /v1/storefronts + bearer token
    P->>P: validate and render property
    P->>D: pull + compose up --wait
    D-->>P: healthy
    P-->>L: ready, reference, image_digest
```

The machine-readable contract is `api/openapi.yaml` in this repository, and it
remains authoritative for request and response shapes.

**Prose documentation lives in the ecosystem documentation:**

- Provisioner API — <https://misaf.github.io/vendra-ecosystem-docs/controller/provisioning>
- Contract rules and limits — <https://misaf.github.io/vendra-ecosystem-docs/api/provisioner>
