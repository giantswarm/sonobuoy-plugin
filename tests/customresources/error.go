package customresources

import (
	"github.com/giantswarm/microerror"
)

var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

var emptyClusterNetworkError = &microerror.Error{
	Kind: "emptyClusterNetworkError",
}
