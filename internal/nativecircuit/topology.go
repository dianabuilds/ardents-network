package nativecircuit

type nativeTopology struct {
	profile          string
	nodeRoles        []string
	applicationRoles []string
	captureRoles     []string
	networkRoles     map[string][]string
}

var c5Topology = nativeTopology{
	profile:          "native",
	nodeRoles:        nativeNodeRoles,
	applicationRoles: nativeApplicationRoles,
	captureRoles:     []string{"user", "user-entry", "user-interior", "rendezvous", "service-interior", "data-service-entry", "introduction-forwarder", "introduction-node", "introduction-interior", "introduction-entry"},
	networkRoles:     nativeNetworkRoles,
}

var c3Topology = nativeTopology{
	profile:          "c3",
	nodeRoles:        []string{"user-entry", "rendezvous", "data-service-entry", "introduction-node"},
	applicationRoles: []string{"user", "service", "user-entry", "rendezvous", "data-service-entry", "introduction-node"},
	captureRoles:     []string{"user", "user-entry", "rendezvous", "data-service-entry"},
	networkRoles: map[string][]string{
		"c3-user-entry":           {"user", "user-entry"},
		"c3-entry-rendezvous":     {"user-entry", "rendezvous"},
		"c3-rendezvous-entry":     {"rendezvous", "data-service-entry"},
		"c3-entry-service":        {"data-service-entry", "service"},
		"c3-user-introduction":    {"user-entry", "introduction-node"},
		"c3-service-introduction": {"data-service-entry", "introduction-node"},
	},
}

var directTopology = nativeTopology{
	profile: "direct", applicationRoles: []string{"user", "service"}, captureRoles: []string{"user"},
	networkRoles: map[string][]string{"direct-link": {"user", "service"}},
}

func topologyFor(workload *nativeWorkload) nativeTopology {
	if workload != nil && workload.Profile == workloadC3 {
		return c3Topology
	}
	if workload != nil && workload.Profile == workloadDirect {
		return directTopology
	}
	return c5Topology
}

func (topology nativeTopology) services() []string {
	services := append([]string(nil), topology.applicationRoles...)
	for _, role := range topology.applicationRoles {
		services = append(services, "shape-"+role)
	}
	for _, role := range topology.captureRoles {
		services = append(services, "capture-"+role)
	}
	return services
}
