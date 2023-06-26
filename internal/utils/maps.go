package utils

func MergeMaps[K comparable, V any](a, b map[K]V) map[K]V {
	m := make(map[K]V)
	for k, v := range a {
		m[k] = v
	}

	for k, v := range b {
		m[k] = v
	}

	return m
}
