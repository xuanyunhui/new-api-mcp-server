package openapi

import _ "embed"

//go:embed api.json
var APISpec []byte

//go:embed relay.json
var RelaySpec []byte
