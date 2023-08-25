package utils

func MergeMaps[K comparable, V any](a, b map[K]V, exclude ...K) map[K]V {
	m := make(map[K]V)
	for k, v := range a {
		if !SliceContains(exclude, k) {
			m[k] = v
		}
	}

	for k, v := range b {
		if !SliceContains(exclude, k) {
			m[k] = v
		}
	}

	return m
}

func ExcludeMapKeys[K comparable, V any](a map[K]V, exclude ...K) map[K]V {
	m := make(map[K]V)
	for k, v := range a {
		if !SliceContains(exclude, k) {
			m[k] = v
		}
	}
	return m
}
