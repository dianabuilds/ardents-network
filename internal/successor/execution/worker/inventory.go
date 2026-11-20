package worker

// inventory selects one closed root-installed worker artifact. It is private to
// this package: no local Application request can select an inventory or path.
// Ordinary Execution admits only the Text inventory through the public Activate;
// the Stream inventory is admitted solely by the trusted qualification launch
// path (ActivateStream), mirroring the predecessor Endpoint's private Inventory
// design. A normal text job can neither select nor reach the stream artifact.
// The concrete inventories describe Linux systemd artifacts and live in
// inventory_linux.go; the type itself is shared because Instance carries it on
// every platform.
type inventory struct {
	prefix       string
	root         string
	manifest     string
	schema       string
	rule         string
	socketDir    string
	userPrefix   string
	slice        string
	cgroupPrefix string
}
