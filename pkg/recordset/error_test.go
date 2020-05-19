package recordset

import (
	"github.com/giantswarm/microerror"
)

var mockClientError = &microerror.Error{
	Kind: "mockClientError",
}

// IsMockClientError asserts mockClientError.
func IsMockClientError(err error) bool { // nolint: unused
	return microerror.Cause(err) == mockClientError
}
