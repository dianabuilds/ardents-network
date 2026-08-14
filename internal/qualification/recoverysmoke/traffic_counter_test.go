package recoverysmoke

import (
	"strings"
	"testing"
)

func TestReadNetworkTrafficSumsNonLoopbackInterfaces(t *testing.T) {
	input := "Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n" +
		" lo: 100 1 0 0 0 0 0 0 100 1 0 0 0 0 0 0\n" +
		" eth0: 200 2 0 0 0 0 0 0 300 3 0 0 0 0 0 0\n" +
		" eth1: 400 4 0 0 0 0 0 0 500 5 0 0 0 0 0 0\n"
	value, err := readNetworkTraffic(strings.NewReader(input))
	if err != nil || value.Kind != "traffic" || value.Interfaces != 2 || value.Received != 600 || value.Sent != 800 {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}

func TestReadNetworkTrafficRejectsMalformedAndMissingInterfaces(t *testing.T) {
	for name, input := range map[string]string{
		"missing":   "Inter-| Receive | Transmit\n lo: 1 1 0 0 0 0 0 0 1 1 0 0 0 0 0 0\n",
		"malformed": "eth0: 1 2\n",
		"counter":   "eth0: nope 1 0 0 0 0 0 0 1 1 0 0 0 0 0 0\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readNetworkTraffic(strings.NewReader(input)); err == nil {
				t.Fatal("invalid network traffic table passed")
			}
		})
	}
}
