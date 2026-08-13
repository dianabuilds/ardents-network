package serviceconn

import "sync"

func newResourceObserver() func(string, int) uint32 {
	var mu sync.Mutex
	current, highWater := map[string]uint32{}, map[string]uint32{}
	return func(kind string, delta int) uint32 {
		mu.Lock()
		defer mu.Unlock()
		if delta > 0 {
			current[kind] += uint32(delta)
			if current[kind] > highWater[kind] {
				highWater[kind] = current[kind]
			}
		} else if delta < 0 && current[kind] >= uint32(-delta) {
			current[kind] -= uint32(-delta)
		}
		return highWater[kind]
	}
}

func acquireResource(observe func(string, int) uint32, kind string) func() {
	observe(kind, 1)
	var once sync.Once
	return func() { once.Do(func() { observe(kind, -1) }) }
}
