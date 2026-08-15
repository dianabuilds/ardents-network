package recoverysmoke

import "path/filepath"

func configureImpairedFixture(root string) error {
	for _, role := range []string{"client", "publisher"} {
		path := filepath.Join(root, "generations", "1", role+".json")
		if err := updatePlan(path, func(plan map[string]any) {
			plan["BytesEachDirection"] = 192 << 20
		}); err != nil {
			return err
		}
	}
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		path := filepath.Join(root, "route", "plans", role+".json")
		if err := updatePlan(path, func(plan map[string]any) {
			delete(plan, "AttachmentPlans")
			delete(plan, "ConcurrentAttachments")
			plan["Attachments"] = 1
			plan["Lifetime"] = "15m"
		}); err != nil {
			return err
		}
	}
	return nil
}
