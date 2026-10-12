package bundledtools

import _ "embed"

//go:embed assets/rg-linux-amd64.gz
var rgData []byte

//go:embed assets/fzf-linux-amd64.gz
var fzfData []byte
