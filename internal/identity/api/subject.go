package api

func NormalizeSubject(call CallContext) Subject {
	return Subject{
		Ref:           call.CanonicalSubject(),
		Authenticated: call.Authenticated,
		Capabilities:  NormalizeCapabilities(call.CanonicalCapabilities(), nil),
	}
}

func NormalizeCapabilities(primary []string, legacy []string) []string {
	if len(primary) > 0 {
		return append([]string(nil), primary...)
	}
	if len(legacy) > 0 {
		return append([]string(nil), legacy...)
	}
	return nil
}
