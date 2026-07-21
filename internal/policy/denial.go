package policy

type Denial struct {
	Code    string
	Message string
}

func newDenial(code, message string) Denial {
	return Denial{Code: code, Message: message}
}

func (d Denial) Empty() bool {
	return d.Code == "" && d.Message == ""
}

func (d Denial) Error() string {
	return d.Code + ": " + d.Message
}
