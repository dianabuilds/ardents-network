package endpoint

// erase overwrites a volatile byte slice once its lifecycle owner has ended.
func erase(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
