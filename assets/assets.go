package assets

import "embed"

//go:embed compose/*/*.yml compose/proxy/dynamic/*.yml
var Files embed.FS
