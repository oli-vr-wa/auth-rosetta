package api

import (
	jsonv2 "encoding/json/v2"
	"io"
)

// decodeJSON parses exactly one JSON value from r into dst. Go 1.27's v2
// decoder is strict by default: invalid UTF-8, duplicate names and trailing
// data are errors. The alias avoids a clash with the v1 package that
// respond.go still uses for output.
func decodeJSON(r io.Reader, dst any) error {
	return jsonv2.UnmarshalRead(r, dst)
}
