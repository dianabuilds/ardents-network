package content

import (
	"ardents/internal/identity/principal"
	"ardents/internal/localapi/rpc"
)

func admittedOwner(call rpc.Call) (principal.ID, *rpc.Error) {
	owner, err := principal.Parse(call.Effective())
	if err != nil {
		return principal.ID{}, &rpc.Error{
			Code: "forbidden", Category: "forbidden", Message: "content owner is invalid",
			Domain: "data", Operation: "data.owner", Reason: "forbidden", Details: map[string]any{},
		}
	}
	return owner, nil
}
