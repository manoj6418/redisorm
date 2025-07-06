package internal

import (
	"strings"
	"sync"
)

// builderPool recycles *strings.Builder to minimise GC pressure during the
// fast-path of query compilation.  Borrow with GetBuilder, return with
// PutBuilder.
//
//	Sb := internal.GetBuilder()
//	defer internal.PutBuilder(sb)
//	Expr.compile(sb)
var builderPool = sync.Pool{
	New: func() any { return new(strings.Builder) },
}

// GetBuilder fetches a cleared *strings.Builder.
func GetBuilder() *strings.Builder {
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	return b
}

// PutBuilder returns a Builder to the pool.  The caller MUST discard its
// reference afterwards — using it again is a data race.
func PutBuilder(b *strings.Builder) { builderPool.Put(b) }
