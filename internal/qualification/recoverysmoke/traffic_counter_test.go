package recoverysmoke

import (
	"context"
	"errors"
	"os"
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

func TestRetainNetworkTrafficReportsLastCompleteHighWater(t *testing.T) {
	reports := make(chan os.Signal, 1)
	reports <- os.Interrupt
	stop := errors.New("stop after receipt")
	var observed trafficCounterReceipt
	err := retainNetworkTraffic(context.Background(), reports, nil, func(value trafficCounterReceipt) error {
		observed = value
		return stop
	}, func() (trafficCounterReceipt, error) {
		return trafficCounterReceipt{Kind: "traffic", Interfaces: 1, Received: 10, Sent: 20}, nil
	})
	if !errors.Is(err, stop) || observed.Received != 10 || observed.Sent != 20 || observed.Interfaces != 1 {
		t.Fatalf("observed=%+v err=%v", observed, err)
	}
}

func TestParseTrafficReceiptRequiresOneTypedReceipt(t *testing.T) {
	value, err := parseTrafficReceipt([]byte("{\"kind\":\"ready\"}\n" +
		"{\"kind\":\"traffic\",\"interfaces\":1,\"received_bytes\":10,\"sent_bytes\":20}\n"))
	if err != nil || value.Received != 10 || value.Sent != 20 {
		t.Fatalf("value=%+v err=%v", value, err)
	}
	for name, raw := range map[string]string{
		"missing traffic": "{\"kind\":\"ready\"}\n",
		"missing ready":   "{\"kind\":\"traffic\",\"interfaces\":1,\"received_bytes\":1,\"sent_bytes\":1}\n",
		"duplicate ready": "{\"kind\":\"ready\"}\n{\"kind\":\"ready\"}\n" +
			"{\"kind\":\"traffic\",\"interfaces\":1,\"received_bytes\":1,\"sent_bytes\":1}\n",
		"duplicate": "{\"kind\":\"ready\"}\n" +
			"{\"kind\":\"traffic\",\"interfaces\":1,\"received_bytes\":1,\"sent_bytes\":1}\n" +
			"{\"kind\":\"traffic\",\"interfaces\":1,\"received_bytes\":2,\"sent_bytes\":2}\n",
		"malformed": "secret text\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseTrafficReceipt([]byte(raw)); err == nil {
				t.Fatal("invalid receipt log passed")
			}
		})
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
