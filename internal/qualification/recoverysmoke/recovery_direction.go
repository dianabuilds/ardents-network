package recoverysmoke

import "path/filepath"

func configureRecoveryDirection(generationRoot, direction string) error {
	clientSend, clientReceive := uint32(4<<20), uint32(0)
	publisherSend, publisherReceive := uint32(0), uint32(4<<20)
	if direction == "publisher-to-client" {
		clientSend, clientReceive = 0, 4<<20
		publisherSend, publisherReceive = 4<<20, 0
	}
	for role, bounds := range map[string][2]uint32{
		"client": {clientSend, clientReceive}, "publisher": {publisherSend, publisherReceive},
	} {
		path := filepath.Join(generationRoot, role+".json")
		if err := updatePlan(path, func(plan map[string]any) {
			delete(plan, "BytesEachDirection")
			plan["SendBytes"], plan["ReceiveBytes"] = bounds[0], bounds[1]
		}); err != nil {
			return err
		}
	}
	return nil
}
