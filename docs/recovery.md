# Recovery

Restore database and controller secrets/ACME data, install the matching signed
`vendra` binary, and run `vendra stack up`. Runtime property directories are
disposable. In Laravel, run `php artisan storefront:reconcile --sync` inside the
platform container to replay the authoritative database fleet through API v1.

