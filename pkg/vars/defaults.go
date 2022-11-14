package vars

import linstor "github.com/LINBIT/golinstor"

var DefaultControllerProperties = map[string]string{
	// TODO: Remove once DRBD does not need this anymore
	linstor.NamespcDrbdNetOptions + "/rr-conflict":                        "retry-connect",
	linstor.NamespcDrbdResourceOptions + "/on-suspended-primary-outdated": "force-secondary",
	linstor.NamespcDrbdResourceOptions + "/on-no-data-accessible":         "suspend-io",
	linstor.NamespcDrbdOptions + "/auto-quorum":                           "suspend-io",
	// The operator can do its own eviction when necessary
	linstor.NamespcDrbdOptions + "/AutoEvictAllowEviction": "false",
}
