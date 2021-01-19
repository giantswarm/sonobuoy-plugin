package client

import (
	"strings"

	"github.com/giantswarm/microerror"
)

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	causes := []string{
		"ResourceGroupNotFound",
	}

	for _, cause := range causes {
		if strings.Contains(microerror.Cause(err).Error(), cause) {
			return true
		}
	}

	return false
}
