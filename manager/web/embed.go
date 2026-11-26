package web

import "embed"

// Dist holds the built Vite frontend.
//
//go:embed all:dist
var Dist embed.FS

//go:embed swagger.json
var swaggerJSON []byte

func SwaggerJSON() ([]byte, error) {
	if len(swaggerJSON) == 0 {
		return nil, embedMissing("swagger.json")
	}
	return swaggerJSON, nil
}

type embedMissing string

func (e embedMissing) Error() string { return "missing embedded file: " + string(e) }
