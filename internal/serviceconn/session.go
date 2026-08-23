package serviceconn

func (endpoint *endpoint) admit(input Request) (Result, error) {
	switch input.Surface {
	case "connection":
	case "administration":
	default:
		return denied("local interface surface is not granted")
	}
	capability, err := endpoint.admission.Admit(input.Principal, input.Surface)
	if err != nil {
		return failed("local authorization or policy denial", err.Error(), err)
	}
	return Result{Class: "authorized", Session: capability}, nil
}

func (endpoint *endpoint) consume(capability, principal [32]byte, surface string) error {
	_, err := endpoint.admission.Consume(capability, principal, surface)
	return err
}
