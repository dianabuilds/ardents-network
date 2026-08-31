package browseradapter

import "testing"

func TestServiceLinkConformanceVectors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		host string
		link string
	}{
		{host: "reference.ard", link: "ardents-alpha://reference"},
		{host: "child.reference.ard", link: "ardents-alpha://child.reference"},
		{host: "a-1.ard", link: "ardents-alpha://a-1"},
		{host: "REFERENCE.ard"},
		{host: "-reference.ard"},
		{host: "reference--site.ard"},
		{host: "123.ard"},
		{host: "reference.example"},
	}
	for _, test := range tests {
		got, err := serviceLinkForHostname(test.host)
		if test.link == "" {
			if err == nil {
				t.Errorf("hostname %q produced %q", test.host, got)
			}
			continue
		}
		if err != nil || got != test.link {
			t.Errorf("hostname %q = %q, %v; want %q", test.host, got, err, test.link)
		}
	}
}
