package route

import "testing"

func TestDeliveryConfirmsIntroductionAdmissionBeforeCredentialIsSpent(t *testing.T) {
	attachment := [32]byte{1}
	cases := []struct {
		name     string
		delivery IntroductionDeliveryResult
		want     bool
	}{
		{name: "delivered exact attachment", delivery: IntroductionDeliveryResult{AttachmentID: attachment, Outcome: IntroductionDelivered}, want: true},
		{name: "unavailable", delivery: IntroductionDeliveryResult{AttachmentID: attachment, Outcome: IntroductionUnavailable}},
		{name: "different attachment", delivery: IntroductionDeliveryResult{AttachmentID: [32]byte{2}, Outcome: IntroductionDelivered}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := deliveryConfirmsIntroductionAdmission(test.delivery, attachment); got != test.want {
				t.Fatalf("delivery confirmation = %t, want %t", got, test.want)
			}
		})
	}
}
