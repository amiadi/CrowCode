package bundledtools

import _ "embed"

//go:embed assets/rg-windows-amd64.gz
var rgData []byte

//go:embed assets/fzf-windows-amd64.gz
var fzfData []byte
