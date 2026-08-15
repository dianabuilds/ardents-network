package route

import "net"

func closeCapacityConnections(listener net.Listener, connections map[net.Conn]struct{}) {
	_ = listener.Close()
	for connection := range connections {
		_ = connection.Close()
	}
}

func drainCapacityConnections(connections map[net.Conn]struct{}, results <-chan capacityResult,
	admission *capacityAdmission, observation *Evidence) {
	for len(connections) > 0 {
		result := <-results
		delete(connections, result.connection)
		if result.authenticated {
			admission.release()
		}
	}
}
