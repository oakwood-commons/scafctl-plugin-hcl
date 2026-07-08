// Copyright 2025-2026 Oakwood Commons
// SPDX-License-Identifier: Apache-2.0

package hcl

import (
	"math/big"

	"github.com/zclconf/go-cty/cty"
)

// ctyStringVal creates a cty string value.
func ctyStringVal(s string) cty.Value {
	return cty.StringVal(s)
}

// ctyBoolVal creates a cty bool value.
func ctyBoolVal(b bool) cty.Value {
	return cty.BoolVal(b)
}

// ctyNumberIntVal creates a cty number value from int64.
func ctyNumberIntVal(n int64) cty.Value {
	return cty.NumberVal(new(big.Float).SetInt64(n))
}

// ctyNumberFloatVal creates a cty number value from float64.
func ctyNumberFloatVal(f float64) cty.Value {
	return cty.NumberFloatVal(f)
}
