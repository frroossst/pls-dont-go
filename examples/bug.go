package examples

import (
	"github.com/apmckinlay/gsuneido/db19/index/iface"
	"github.com/apmckinlay/gsuneido/db19/index/ixbuf"
	"github.com/apmckinlay/gsuneido/util/assert"
	"github.com/apmckinlay/gsuneido/util/generic/slc"
)

// Overlay is the composite in-memory representation of an index
// @immutable
type Overlay struct {
	// bt is the stored base btree (immutable)
	bt iface.Btree
	// mut is the per transaction mutable top ixbuf.T, nil if read-only
	mut *ixbuf.T
	// layers is: (immutable)
	// - a base ixbuf of merged but not persisted changes,
	// - plus ixbuf's from completed but un-merged transactions
	layers []*ixbuf.T
}

// UpdateWith combines the overlay result of a transaction
// with the latest overlay. It is called by Meta.LayeredOnto.
// The immutable part of ov was taken at the start of the transaction
// so it will be out of date.
// The checker ensures that the updates are independent.
func (ov *Overlay) UpdateWith(latest *Overlay) {
	ov.bt = latest.bt                           // @allow-mutate
	ov.layers = slc.With(latest.layers, ov.mut) // @allow-mutate
	ov.mut = nil                                // @allow-mutate
	assert.That(len(ov.layers) >= 2)
}

