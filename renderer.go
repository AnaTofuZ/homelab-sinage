package main

import (
	_ "embed"
	"fmt"

	bf "github.com/barefootjs/runtime/bf"
)

//go:embed layout.html
var layoutHTML string

// defaultLayout is the wrapping HTML the BarefootJS render pipeline
// hands every component. Edit freely — this file is yours.
func defaultLayout(ctx *bf.RenderContext) string {
	return fmt.Sprintf(layoutHTML,
		ctx.Title,
		ctx.ComponentHTML,
		ctx.Portals,
		ctx.Scripts,
	)
}
