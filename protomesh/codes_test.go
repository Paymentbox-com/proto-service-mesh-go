package protomesh_test

import (
	"testing"

	"github.com/Paymentbox-com/proto-service-mesh-go/protomesh"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

func TestCodeConstructorCarriesMessageAndDetails(t *testing.T) {
	me := protomesh.NewNotFoundError("no such key", &errdetails.ErrorInfo{Reason: "GONE"})

	if me.Code() != code.Code_NOT_FOUND {
		t.Errorf("Code() = %v, want NOT_FOUND", me.Code())
	}
	if me.Message() != "no such key" {
		t.Errorf("Message() = %q, want %q", me.Message(), "no such key")
	}
	if len(me.Details()) != 1 {
		t.Errorf("Details() has %d entries, want 1", len(me.Details()))
	}
}

func TestEachCodeConstructorSetsItsCode(t *testing.T) {
	check := func(name string, me *protomesh.MeshError, want code.Code) {
		if me.Code() != want {
			t.Errorf("%s: Code() = %v, want %v", name, me.Code(), want)
		}
	}
	check("Cancelled", protomesh.NewCancelledError(""), code.Code_CANCELLED)
	check("Unknown", protomesh.NewUnknownError(""), code.Code_UNKNOWN)
	check("InvalidArgument", protomesh.NewInvalidArgumentError(""), code.Code_INVALID_ARGUMENT)
	check("DeadlineExceeded", protomesh.NewDeadlineExceededError(""), code.Code_DEADLINE_EXCEEDED)
	check("NotFound", protomesh.NewNotFoundError(""), code.Code_NOT_FOUND)
	check("AlreadyExists", protomesh.NewAlreadyExistsError(""), code.Code_ALREADY_EXISTS)
	check("PermissionDenied", protomesh.NewPermissionDeniedError(""), code.Code_PERMISSION_DENIED)
	check("Unauthenticated", protomesh.NewUnauthenticatedError(""), code.Code_UNAUTHENTICATED)
	check("ResourceExhausted", protomesh.NewResourceExhaustedError(""), code.Code_RESOURCE_EXHAUSTED)
	check("FailedPrecondition", protomesh.NewFailedPreconditionError(""), code.Code_FAILED_PRECONDITION)
	check("Aborted", protomesh.NewAbortedError(""), code.Code_ABORTED)
	check("OutOfRange", protomesh.NewOutOfRangeError(""), code.Code_OUT_OF_RANGE)
	check("Unimplemented", protomesh.NewUnimplementedError(""), code.Code_UNIMPLEMENTED)
	check("Internal", protomesh.NewInternalError(""), code.Code_INTERNAL)
	check("Unavailable", protomesh.NewUnavailableError(""), code.Code_UNAVAILABLE)
	check("DataLoss", protomesh.NewDataLossError(""), code.Code_DATA_LOSS)
}

func TestCodeConstantsAreTheGoogleRpcCodes(t *testing.T) {
	if protomesh.OK != code.Code_OK || protomesh.NotFound != code.Code_NOT_FOUND {
		t.Errorf("constants differ from google.rpc.Code")
	}
}
