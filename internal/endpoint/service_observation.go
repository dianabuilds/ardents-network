package endpoint

func (endpoint *endpoint) observe(result *RuntimeResult) {
	result.AcceptedIPCHighWater = endpoint.resources("accepted-ipc", 0)
}
