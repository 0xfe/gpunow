package gcp

import (
	"errors"

	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func IsNotFound(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 404
	}
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPCode() == 404 {
			return true
		}
		if st := apiErr.GRPCStatus(); st != nil && st.Code() == codes.NotFound {
			return true
		}
	}
	if status.Code(err) == codes.NotFound {
		return true
	}
	return false
}

func IsAlreadyExists(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 409
	}
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPCode() == 409 {
			return true
		}
		if st := apiErr.GRPCStatus(); st != nil && st.Code() == codes.AlreadyExists {
			return true
		}
	}
	if status.Code(err) == codes.AlreadyExists {
		return true
	}
	return false
}
