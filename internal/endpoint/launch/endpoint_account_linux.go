package launch

import (
	"os/user"

	"strconv"
)

// Native account mechanisms are shared by initial preparation and independent
// installed inspection. Callers retain effect admission and exact expected IDs;
// these functions grant no journal, lease, recovery or startup authority.

// freeEndpointAccountID selects one unused 100-999 identity for both the user
// and its private group. It refuses an already-present endpoint account; it
// creates nothing and grants no authority.

func observeEndpointAccount() (uint32, uint32, error) {
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		return 0, 0, err
	}
	uid, uidErr := strconv.ParseUint(account.Uid, 10, 32)
	gid, gidErr := strconv.ParseUint(account.Gid, 10, 32)
	group, groupErr := user.LookupGroup("ardents-endpoint")
	groups, groupsErr := account.GroupIds()
	if uidErr != nil || gidErr != nil || uid == 0 || gid == 0 || uid == 1<<32-1 || gid == 1<<32-1 ||
		groupErr != nil || group.Gid != account.Gid || groupsErr != nil || len(groups) != 1 || groups[0] != account.Gid {
		return 0, 0, ErrNativeUnavailable
	}
	return uint32(uid), uint32(gid), nil
}
