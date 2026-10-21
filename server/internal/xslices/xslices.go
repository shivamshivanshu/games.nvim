// Package xslices holds the projections the server repeats across layers.
package xslices

import "maps"

// Map never returns nil, so a projected wire array encodes as [] and not null.
func Map[S ~[]E, E, R any](s S, f func(E) R) []R {
	out := make([]R, 0, len(s))
	for _, e := range s {
		out = append(out, f(e))
	}
	return out
}

func MapErr[S ~[]E, E, R any](s S, f func(E) (R, error)) ([]R, error) {
	out := make([]R, 0, len(s))
	for _, e := range s {
		r, err := f(e)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// MapValues keeps a nil map nil, so an absent map stays absent.
func MapValues[M ~map[K]V, K comparable, V, R any](m M, f func(V) R) map[K]R {
	if m == nil {
		return nil
	}
	out := make(map[K]R, len(m))
	for k, v := range m {
		out[k] = f(v)
	}
	return out
}

// With returns a copy of m with k set to v, leaving m untouched.
func With[M ~map[K]V, K comparable, V any](m M, k K, v V) M {
	out := maps.Clone(m)
	if out == nil {
		out = make(M, 1)
	}
	out[k] = v
	return out
}

// Clone never returns nil, unlike slices.Clone, so a copied wire array
// encodes as [] and not null.
func Clone[S ~[]E, E any](s S) S {
	return append(make(S, 0, len(s)), s...)
}
