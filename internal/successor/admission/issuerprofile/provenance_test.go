package issuerprofile_test

import (
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

// A prepared request has passed public grammar checks but has no verified Node
// signature. Go type conversion must not turn that weaker evidence into proof.
func TestPreparedRequestCannotBecomeVerifiedByConversion(t *testing.T) {
	request := reflect.TypeFor[issuerprofile.Request]()
	verified := reflect.TypeFor[issuerprofile.Verified]()
	if request.ConvertibleTo(verified) {
		t.Fatal("a caller can convert Prepare output to Verified without checking a signature")
	}
}
