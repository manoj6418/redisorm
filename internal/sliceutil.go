// internal/sliceutil.go
//
//
//   • Pure – no side effects.
//   • Safe – never modify the input slice in-place.
//   • Generic – work with any comparable / ordered type.
//   • Allocation-friendly – minimal copies.
// ----------------------------------------------------------------------------

package internal

import "golang.org/x/exp/constraints"

// ---------------------------------------------------------------------
// Basic predicates / membership
// ---------------------------------------------------------------------

// Contains reports whether v E xs (O(n)).
func Contains[T comparable](xs []T, v T) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// All returns true if every element satisfies pred.
func All[T any](xs []T, pred func(T) bool) bool {
	for _, x := range xs {
		if !pred(x) {
			return false
		}
	}
	return true
}

// Any returns true if at least one element satisfies pred.
func Any[T any](xs []T, pred func(T) bool) bool {
	for _, x := range xs {
		if pred(x) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Transformations
// ---------------------------------------------------------------------

// Map applies f to each element and returns a new slice.
func Map[A any, B any](xs []A, f func(A) B) []B {
	out := make([]B, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

// Filter keeps values where pred(x) == true.
func Filter[T any](xs []T, pred func(T) bool) []T {
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if pred(x) {
			out = append(out, x)
		}
	}
	return out
}

// Reduce folds xs into an accumulator acc with f.
func Reduce[T any, R any](xs []T, acc R, f func(R, T) R) R {
	for _, x := range xs {
		acc = f(acc, x)
	}
	return acc
}

// Flatten flattens [][]T → []T.
func Flatten[T any](xss [][]T) []T {
	var total int
	for _, xs := range xss {
		total += len(xs)
	}
	out := make([]T, 0, total)
	for _, xs := range xss {
		out = append(out, xs...)
	}
	return out
}

// ---------------------------------------------------------------------
// Set-like helpers (require comparable)
// ---------------------------------------------------------------------

// Unique dedups while preserving first-seen order.
func Unique[T comparable](xs []T) []T {
	seen := make(map[T]struct{}, len(xs))
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; !ok {
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	return out
}

// Intersect returns elements common to both slices.
func Intersect[T comparable](a, b []T) []T {
	set := make(map[T]struct{}, len(a))
	for _, x := range a {
		set[x] = struct{}{}
	}
	var out []T
	for _, x := range b {
		if _, ok := set[x]; ok {
			out = append(out, x)
		}
	}
	return Unique(out)
}

// Difference returns items in a that are NOT in b.
func Difference[T comparable](a, b []T) []T {
	set := make(map[T]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}
	var out []T
	for _, x := range a {
		if _, ok := set[x]; !ok {
			out = append(out, x)
		}
	}
	return out
}

// Union merges two slices and dedups.
func Union[T comparable](a, b []T) []T { return Unique(append(a, b...)) }

// ---------------------------------------------------------------------
// Chunking & order
// ---------------------------------------------------------------------

// Chunk splits xs into sub-slices of size =< n.
func Chunk[T any](xs []T, n int) [][]T {
	if n <= 0 {
		return nil
	}
	var out [][]T
	for i := 0; i < len(xs); i += n {
		end := i + n
		if end > len(xs) {
			end = len(xs)
		}
		out = append(out, xs[i:end])
	}
	return out
}

// Reverse builds a new slice in reverse order.
func Reverse[T any](xs []T) []T {
	out := make([]T, len(xs))
	for i, x := range xs {
		out[len(xs)-1-i] = x
	}
	return out
}

// ---------------------------------------------------------------------
// Numeric helpers (ordered constraint: ints, floats, strings, …)
// ---------------------------------------------------------------------

// Min returns the smallest element. Panics on empty slice.
func Min[T constraints.Ordered](xs []T) T {
	if len(xs) == 0 {
		panic("sliceutil.Min: empty slice")
	}
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

// Max returns the largest element. Panics on empty slice.
func Max[T constraints.Ordered](xs []T) T {
	if len(xs) == 0 {
		panic("sliceutil.Max: empty slice")
	}
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

// Sum accumulates numeric values (int, float…).
func Sum[T constraints.Integer | constraints.Float](xs []T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

// ReverseInPlace reverses xs without allocating.
func ReverseInPlace[T any](xs []T) {
	lo, hi := 0, len(xs)-1
	for lo < hi {
		xs[lo], xs[hi] = xs[hi], xs[lo]
		lo++
		hi--
	}
}
