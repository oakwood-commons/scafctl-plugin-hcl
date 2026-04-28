// Package main is the entry point for the scafctl-plugin-hcl plugin.
package main

import (
	"github.com/oakwood-commons/scafctl-plugin-hcl/internal/hcl"

	sdkplugin "github.com/oakwood-commons/scafctl-plugin-sdk/plugin"
)

func main() {
	sdkplugin.Serve(hcl.NewPlugin())
}
