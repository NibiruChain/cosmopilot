package utils

func SliceContains[K comparable](l []K, key K) bool {
	for _, k := range l {
		if k == key {
			return true
		}
	}
	return false
}
