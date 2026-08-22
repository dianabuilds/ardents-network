package updatetransaction

type selfTestUnavailable struct{}

func (selfTestUnavailable) Error() string { return "self-test network unavailable" }

// ErrSelfTestUnavailable is the only machine-readable observation that a
// normatively required network self-test could not complete. It deliberately
// carries no endpoint, timing, or mutable caller data.
var ErrSelfTestUnavailable error = selfTestUnavailable{}

// selfTestUnavailableOnly accepts an error tree only when every non-nil leaf
// is ErrSelfTestUnavailable. Bounds make hostile cyclic or exploding Unwrap
// implementations a local failure, and recover keeps a panicking caller-owned
// error implementation from escaping the transaction boundary.
func selfTestUnavailableOnly(err error) (available bool) {
	defer func() {
		if recover() != nil {
			available = false
		}
	}()
	var nodes, edges uint8
	var walk func(error) bool
	walk = func(current error) bool {
		if current == nil {
			return false
		}
		nodes++
		if nodes > 16 {
			return false
		}
		if _, ok := current.(selfTestUnavailable); ok {
			return true
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			children := joined.Unwrap()
			hasChild := false
			for _, child := range children {
				if child == nil {
					continue
				}
				edges++
				if edges > 16 || !walk(child) {
					return false
				}
				hasChild = true
			}
			return hasChild
		}
		if wrapped, ok := current.(interface{ Unwrap() error }); ok {
			edges++
			return edges <= 16 && walk(wrapped.Unwrap())
		}
		return false
	}
	return walk(err)
}
