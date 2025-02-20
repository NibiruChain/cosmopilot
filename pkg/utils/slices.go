package utils

func SliceContains[K comparable](l []K, key K) bool {
	for _, k := range l {
		if k == key {
			return true
		}
	}
	return false
}

func SliceContainsObj[K comparable](l []K, obj K, f func(K, K) bool) bool {
	for _, k := range l {
		if f(k, obj) {
			return true
		}
	}
	return false
}
