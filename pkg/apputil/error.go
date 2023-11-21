package apputil

import (
	"github.com/giantswarm/microerror"
	"github.com/google/go-github/v45/github"
)

var appFailedError = &microerror.Error{
	Kind: "appFailedError",
}

var appNotReadyError = &microerror.Error{
	Kind: "appNotReadyError",
}

func IsGithubNotFound(err error) bool {
	if err == nil {
		return false
	}

	v, ok := err.(*github.ErrorResponse)
	if !ok {
		return false
	}

	return v.Message == "Not Found"
}
