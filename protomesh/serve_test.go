package protomesh_test

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/Paymentbox-com/proto-service-mesh-go/internal/testproto/shop"
	"github.com/Paymentbox-com/proto-service-mesh-go/protomesh"
	"github.com/Paymentbox-com/service-mesh-go/mesh"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"
)

var garbage = []byte{0xff, 0xff, 0xff, 0xff}

// statusOf decodes a reply carrying a MeshError.
func statusOf(t *testing.T, reply mesh.Message) *status.Status {
	t.Helper()
	var st status.Status
	if err := proto.Unmarshal(reply.Payload, &st); err != nil {
		t.Fatal(err)
	}
	return &st
}

func TestEndpointRepliesWithTheEncodedResponse(t *testing.T) {
	var got *shop.Order
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(_ context.Context, req *shop.Order) (*shop.Order, error) {
		got = req
		return order("reply"), nil
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{Payload: mustMarshal(t, order("ask"))})
	if err != nil {
		t.Fatal(err)
	}

	if !ep.Target.Equal(shop.OrderTargets.Place) {
		t.Errorf("endpoint target = %v", ep.Target)
	}
	if got.GetId() != "ask" {
		t.Errorf("handler received %v", got)
	}
	var resp shop.Order
	if err := proto.Unmarshal(reply.Payload, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.GetId() != "reply" {
		t.Errorf("reply decodes to %v", &resp)
	}
	if reply.Metadata[protomesh.MeshContentTypeKey] != protomesh.ContentTypeProtobuf {
		t.Errorf("reply metadata = %v, want Mesh-Content-Type", reply.Metadata)
	}
	if _, ok := reply.Metadata[protomesh.MeshStatusKey]; ok {
		t.Error("a successful reply carries Mesh-Status")
	}
}

func TestEndpointExposesTheIncomingMetadata(t *testing.T) {
	var seen map[string]string
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(ctx context.Context, _ *shop.Order) (*shop.Order, error) {
		seen = protomesh.IncomingMetadata(ctx)
		return &shop.Order{}, nil
	})

	if _, err := ep.Handler(context.Background(), mesh.Message{Metadata: map[string]string{"Request-Id": "7"}}); err != nil {
		t.Fatal(err)
	}

	if seen["Request-Id"] != "7" {
		t.Errorf("IncomingMetadata = %v", seen)
	}
}

func TestEndpointRepliesAMeshErrorAsAStatus(t *testing.T) {
	me := protomesh.NewMeshError(code.Code_NOT_FOUND, "no such key", &errdetails.ErrorInfo{Reason: "GONE"})
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) {
		return nil, me
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata[protomesh.MeshStatusKey] != "5" {
		t.Errorf("Mesh-Status = %q, want 5", reply.Metadata[protomesh.MeshStatusKey])
	}
	if reply.Metadata[protomesh.MeshContentTypeKey] != protomesh.ContentTypeProtobuf {
		t.Errorf("reply metadata = %v, want Mesh-Content-Type", reply.Metadata)
	}
	st := statusOf(t, reply)
	if st.GetCode() != 5 || st.GetMessage() != "no such key" || len(st.GetDetails()) != 1 {
		t.Errorf("status = %v", st)
	}
}

func TestEndpointRepliesUnknownForAnyOtherError(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) {
		return nil, errors.New("database down")
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata[protomesh.MeshStatusKey] != "2" {
		t.Errorf("Mesh-Status = %q, want 2", reply.Metadata[protomesh.MeshStatusKey])
	}
	st := statusOf(t, reply)
	if st.GetCode() != 2 || st.GetMessage() != "database down" {
		t.Errorf("status = %v", st)
	}
}

func TestEndpointRepliesUnknownForAPanic(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) {
		panic("boom")
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata[protomesh.MeshStatusKey] != "2" {
		t.Errorf("Mesh-Status = %q, want 2", reply.Metadata[protomesh.MeshStatusKey])
	}
	if st := statusOf(t, reply); !strings.Contains(st.GetMessage(), "boom") {
		t.Errorf("status message = %q, want the panic value in it", st.GetMessage())
	}
}

func TestEndpointRepliesWithTheReplyMetadataTheHandlerSets(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(ctx context.Context, _ *shop.Order) (*shop.Order, error) {
		protomesh.SetReplyMetadata(ctx, map[string]string{"Request-Id": "7", "Region": "east"})
		protomesh.SetReplyMetadata(ctx, map[string]string{"Region": "west"})
		return order("reply"), nil
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata["Request-Id"] != "7" || reply.Metadata["Region"] != "west" {
		t.Errorf("reply metadata = %v, want Request-Id 7 and the later Region west", reply.Metadata)
	}
	if reply.Metadata[protomesh.MeshContentTypeKey] != protomesh.ContentTypeProtobuf {
		t.Errorf("reply metadata = %v, want Mesh-Content-Type", reply.Metadata)
	}
}

func TestEndpointRepliesAMeshErrorWithTheReplyMetadataTheHandlerSets(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(ctx context.Context, _ *shop.Order) (*shop.Order, error) {
		protomesh.SetReplyMetadata(ctx, map[string]string{"Retry-After": "30"})
		return nil, protomesh.NewNotFoundError("no such key")
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata["Retry-After"] != "30" {
		t.Errorf("reply metadata = %v, want Retry-After", reply.Metadata)
	}
	if reply.Metadata[protomesh.MeshStatusKey] != "5" {
		t.Errorf("Mesh-Status = %q, want 5", reply.Metadata[protomesh.MeshStatusKey])
	}
}

func TestEndpointDropsOptionKeysTheHandlerSetsOnItsReply(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(ctx context.Context, _ *shop.Order) (*shop.Order, error) {
		protomesh.SetReplyMetadata(ctx, map[string]string{"Request-Id": "7", "Mesh-Option-request_timeout": "2s"})
		return order("reply"), nil
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata["Request-Id"] != "7" {
		t.Errorf("reply metadata = %v, want Request-Id", reply.Metadata)
	}
	if v, ok := reply.Metadata["Mesh-Option-request_timeout"]; ok {
		t.Errorf("reply metadata carries Mesh-Option-request_timeout = %q, want it dropped", v)
	}
}

func TestEndpointReplacesTheWireKeysTheHandlerSetsOnAMeshErrorReply(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(ctx context.Context, _ *shop.Order) (*shop.Order, error) {
		protomesh.SetReplyMetadata(ctx, map[string]string{
			protomesh.MeshContentTypeKey: "text/plain", protomesh.MeshStatusKey: "0",
			"Content-Type": "text/plain", "Grpc-Status": "0",
		})
		return nil, protomesh.NewNotFoundError("no such key")
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata[protomesh.MeshContentTypeKey] != protomesh.ContentTypeProtobuf {
		t.Errorf("Mesh-Content-Type = %q, want the package's value over the handler's", reply.Metadata[protomesh.MeshContentTypeKey])
	}
	if reply.Metadata[protomesh.MeshStatusKey] != "5" {
		t.Errorf("Mesh-Status = %q, want the package's 5 over the handler's", reply.Metadata[protomesh.MeshStatusKey])
	}
	for _, k := range []string{"Content-Type", "Grpc-Status"} {
		if v, ok := reply.Metadata[k]; ok {
			t.Errorf("%s = %q, want the handler's value dropped", k, v)
		}
	}
}

func TestEndpointReplacesTheWireKeysTheHandlerSetsOnASuccessfulReply(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(ctx context.Context, _ *shop.Order) (*shop.Order, error) {
		protomesh.SetReplyMetadata(ctx, map[string]string{
			protomesh.MeshContentTypeKey: "text/plain", protomesh.MeshStatusKey: "5",
			"Content-Type": "text/plain", "Grpc-Status": "5",
		})
		return order("reply"), nil
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata[protomesh.MeshContentTypeKey] != protomesh.ContentTypeProtobuf {
		t.Errorf("Mesh-Content-Type = %q, want the package's value over the handler's", reply.Metadata[protomesh.MeshContentTypeKey])
	}
	for _, k := range []string{protomesh.MeshStatusKey, "Content-Type", "Grpc-Status"} {
		if v, ok := reply.Metadata[k]; ok {
			t.Errorf("%s = %q on a successful reply, want none", k, v)
		}
	}
}

func TestEndpointRepliesWithContentTypeToARequestFromAnOlderVersion(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) {
		return order("reply"), nil
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{Metadata: map[string]string{"Content-Type": protomesh.ContentTypeProtobuf}})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Metadata[protomesh.MeshContentTypeKey] != protomesh.ContentTypeProtobuf || reply.Metadata["Content-Type"] != protomesh.ContentTypeProtobuf {
		t.Errorf("reply metadata = %v, want Mesh-Content-Type and Content-Type", reply.Metadata)
	}
	for _, k := range []string{protomesh.MeshStatusKey, "Grpc-Status"} {
		if v, ok := reply.Metadata[k]; ok {
			t.Errorf("%s = %q on a successful reply, want none", k, v)
		}
	}
}

func TestEndpointRepliesAMeshErrorWithGrpcStatusToARequestFromAnOlderVersion(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) {
		return nil, protomesh.NewNotFoundError("no such key")
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{Metadata: map[string]string{"Content-Type": protomesh.ContentTypeProtobuf}})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		protomesh.MeshContentTypeKey: protomesh.ContentTypeProtobuf, "Content-Type": protomesh.ContentTypeProtobuf,
		protomesh.MeshStatusKey: "5", "Grpc-Status": "5",
	}
	if !maps.Equal(reply.Metadata, want) {
		t.Errorf("reply metadata = %v, want %v", reply.Metadata, want)
	}
}

func TestEndpointWarnsOnceForRequestsFromAnOlderVersion(t *testing.T) {
	logs := captureWarnings(t)
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) {
		return &shop.Order{}, nil
	})
	old := mesh.Message{Metadata: map[string]string{"Content-Type": protomesh.ContentTypeProtobuf}}

	for range 2 {
		if _, err := ep.Handler(context.Background(), old); err != nil {
			t.Fatal(err)
		}
	}

	want := `level=WARN msg="protomesh: message carries a deprecated metadata key; the sender runs an older version" key=Content-Type replacement=Mesh-Content-Type target="[shop OrderService Place]"` + "\n"
	if logs.String() != want {
		t.Errorf("logged %q, want %q", logs.String(), want)
	}
}

func TestEndpointRepliesInternalForAnUndecodableRequest(t *testing.T) {
	called := false
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) {
		called = true
		return nil, nil
	})

	reply, err := ep.Handler(context.Background(), mesh.Message{Payload: garbage})
	if err != nil {
		t.Fatal(err)
	}

	if called {
		t.Error("handler ran on an undecodable request")
	}
	if reply.Metadata[protomesh.MeshStatusKey] != "13" {
		t.Errorf("Mesh-Status = %q, want 13", reply.Metadata[protomesh.MeshStatusKey])
	}
	if st := statusOf(t, reply); st.GetCode() != 13 || !strings.HasPrefix(st.GetMessage(), "request does not decode as shop.Order: ") {
		t.Errorf("status = %v, want INTERNAL naming the request type", st)
	}
}

func TestSubscriberDeliversTheDecodedMessageAndMetadata(t *testing.T) {
	var got *shop.Order
	var seen map[string]string
	sub := protomesh.NewSubscriber(shop.OrderTargets.Placed, "", func(ctx context.Context, req *shop.Order) error {
		got, seen = req, protomesh.IncomingMetadata(ctx)
		return nil
	})

	err := sub.Handler(context.Background(), mesh.Message{
		Metadata: map[string]string{"Event-Id": "9"},
		Payload:  mustMarshal(t, order("made")),
	})

	if err != nil {
		t.Fatal(err)
	}
	if !sub.Target.Equal(shop.OrderTargets.Placed) {
		t.Errorf("subscriber target = %v", sub.Target)
	}
	if got.GetId() != "made" {
		t.Errorf("handler received %v", got)
	}
	if seen["Event-Id"] != "9" {
		t.Errorf("IncomingMetadata = %v", seen)
	}
}

func TestSubscriberReturnsTheHandlerErrorUnchanged(t *testing.T) {
	failure := errors.New("cannot record")
	sub := protomesh.NewSubscriber(shop.OrderTargets.Placed, "", func(context.Context, *shop.Order) error {
		return failure
	})

	err := sub.Handler(context.Background(), mesh.Message{})

	if err != failure {
		t.Errorf("err = %v, want the handler's error itself", err)
	}
}

func TestSubscriberReturnsTheDecodeErrorWithoutRunningTheHandler(t *testing.T) {
	called := false
	sub := protomesh.NewSubscriber(shop.OrderTargets.Placed, "", func(context.Context, *shop.Order) error {
		called = true
		return nil
	})

	err := sub.Handler(context.Background(), mesh.Message{Payload: garbage})

	if err == nil || !strings.HasPrefix(err.Error(), "request does not decode as shop.Order: ") {
		t.Fatalf("err = %v, want the decode error naming the message type", err)
	}
	if called {
		t.Error("handler ran on an undecodable message")
	}
}

func TestSetReplyMetadataInASubscriberHasNoEffect(t *testing.T) {
	sub := protomesh.NewSubscriber(shop.OrderTargets.Placed, "", func(ctx context.Context, _ *shop.Order) error {
		protomesh.SetReplyMetadata(ctx, map[string]string{"Request-Id": "7"})
		return nil
	})

	err := sub.Handler(context.Background(), mesh.Message{})

	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestIncomingMetadataOutsideAHandlerIsNil(t *testing.T) {
	if md := protomesh.IncomingMetadata(context.Background()); md != nil {
		t.Errorf("IncomingMetadata = %v, want nil", md)
	}
}

func TestNewEndpointAndNewSubscriberPutTheConsumerGroupInMetadata(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "orders", func(context.Context, *shop.Order) (*shop.Order, error) { return nil, nil })
	sub := protomesh.NewSubscriber(shop.OrderTargets.Placed, "audit", func(context.Context, *shop.Order) error { return nil })

	if got := ep.Metadata[mesh.ConsumerGroupKey]; got != "orders" {
		t.Errorf("endpoint consumer_group = %q, want orders", got)
	}
	if got := sub.Metadata[mesh.ConsumerGroupKey]; got != "audit" {
		t.Errorf("subscriber consumer_group = %q, want audit", got)
	}
}

func TestNewEndpointAndNewSubscriberWithoutAConsumerGroupSetNoMetadata(t *testing.T) {
	ep := protomesh.NewEndpoint(shop.OrderTargets.Place, "", func(context.Context, *shop.Order) (*shop.Order, error) { return nil, nil })
	sub := protomesh.NewSubscriber(shop.OrderTargets.Placed, "", func(context.Context, *shop.Order) error { return nil })

	if _, set := ep.Metadata[mesh.ConsumerGroupKey]; set {
		t.Errorf("endpoint metadata = %v, want no consumer_group", ep.Metadata)
	}
	if _, set := sub.Metadata[mesh.ConsumerGroupKey]; set {
		t.Errorf("subscriber metadata = %v, want no consumer_group", sub.Metadata)
	}
}

func TestSubscriberWarnsOnceForMessagesFromAnOlderVersion(t *testing.T) {
	logs := captureWarnings(t)
	sub := protomesh.NewSubscriber(shop.OrderTargets.Placed, "", func(context.Context, *shop.Order) error { return nil })
	old := mesh.Message{Metadata: map[string]string{"Content-Type": protomesh.ContentTypeProtobuf}}

	for range 2 {
		if err := sub.Handler(context.Background(), old); err != nil {
			t.Fatal(err)
		}
	}

	want := `level=WARN msg="protomesh: message carries a deprecated metadata key; the sender runs an older version" key=Content-Type replacement=Mesh-Content-Type target="[shop OrderService Placed]"` + "\n"
	if logs.String() != want {
		t.Errorf("logged %q, want %q", logs.String(), want)
	}
}
