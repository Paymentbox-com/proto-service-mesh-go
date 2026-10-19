package protomesh

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Paymentbox-com/service-mesh-go/mesh"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"
)

// NewEndpoint wraps fn as the mesh.Endpoint for a ROUTE target. consumerGroup
// is the method's consumer_group option, or "" when it has none, and goes in
// the Endpoint's metadata. The handler
// decodes the request into a fresh Req, exposes the message metadata through
// IncomingMetadata, and replies with the encoded Resp under Mesh-Content-Type
// application/x-protobuf.
//
// The reply carries the metadata fn sets with SetReplyMetadata, without keys
// that start with OptionPrefix, on a successful reply and on a MeshError
// reply. The package sets Mesh-Content-Type, Mesh-Status, Content-Type, and
// Grpc-Status itself and drops the application's values for them, and a
// successful reply carries no status.
//
// A *MeshError from fn becomes a reply whose payload is the encoded
// google.rpc.Status and whose Mesh-Status metadata is the code as a decimal
// string.
//
// A request that carries Content-Type without Mesh-Content-Type comes from
// an older version, and its reply also carries Content-Type and, for a
// MeshError, Grpc-Status. Any other error, or a panic, is reported the same way as UNKNOWN
// with the failure's text. A request that does not decode, or a response
// that does not encode, is reported as INTERNAL with a message naming the
// message type. The handler never returns an error to the runtime.
func NewEndpoint[Req, Resp proto.Message](t mesh.Target, consumerGroup string, fn func(context.Context, Req) (Resp, error)) mesh.Endpoint {
	return mesh.Endpoint{
		Target:   t,
		Metadata: consumerGroupMetadata(consumerGroup),
		Handler: func(ctx context.Context, m mesh.Message) (mesh.Message, error) {
			legacy := legacyContentType(t, m.Metadata)
			req, err := decode[Req]("request", m.Payload)
			if err != nil {
				return statusReply(NewMeshError(code.Code_INTERNAL, err.Error()), nil, legacy), nil
			}
			reply := map[string]string{}
			ctx = withHandlerReply(withIncomingMetadata(ctx, m.Metadata), reply)
			resp, err := serve(ctx, req, fn)
			if err != nil {
				return statusReply(asMeshError(err), reply, legacy), nil
			}
			payload, err := proto.Marshal(resp)
			if err != nil {
				return statusReply(NewMeshError(code.Code_INTERNAL, fmt.Sprintf("response does not encode as %s: %v", resp.ProtoReflect().Descriptor().FullName(), err)), reply, legacy), nil
			}
			return mesh.Message{Metadata: replyMetadata(reply, legacy), Payload: payload}, nil
		},
	}
}

// NewSubscriber wraps fn as the mesh.Subscriber for a TOPIC target.
// consumerGroup is the method's consumer_group option, or "" when it has
// none, and goes in the Subscriber's metadata. The
// handler decodes the message into a fresh Req, exposes the message metadata
// through IncomingMetadata, and returns fn's error to the runtime unchanged.
// A message that carries Content-Type without Mesh-Content-Type logs a
// warning on the first such message for t.
// A message that does not decode returns an error naming the message type.
// Panics are not recovered.
func NewSubscriber[Req proto.Message](t mesh.Target, consumerGroup string, fn func(context.Context, Req) error) mesh.Subscriber {
	return mesh.Subscriber{
		Target:   t,
		Metadata: consumerGroupMetadata(consumerGroup),
		Handler: func(ctx context.Context, m mesh.Message) error {
			legacyContentType(t, m.Metadata)
			req, err := decode[Req]("request", m.Payload)
			if err != nil {
				return err
			}
			return fn(withIncomingMetadata(ctx, m.Metadata), req)
		},
	}
}

// serve runs fn and turns a panic into an error.
func serve[Req, Resp proto.Message](ctx context.Context, req Req, fn func(context.Context, Req) (Resp, error)) (resp Resp, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return fn(ctx, req)
}

// asMeshError returns err as a *MeshError, or UNKNOWN with err's text.
func asMeshError(err error) *MeshError {
	var me *MeshError
	if errors.As(err, &me) {
		return me
	}
	return NewMeshError(code.Code_UNKNOWN, err.Error())
}

// statusReply encodes me as a reply message whose metadata is
// replyMetadata(md, legacy) with Mesh-Status set, and Grpc-Status too when
// legacy is true.
func statusReply(me *MeshError, md map[string]string, legacy bool) mesh.Message {
	payload, err := proto.Marshal(me.Proto())
	if err != nil {
		// A Status holding only a code and text always encodes.
		me = NewMeshError(code.Code_INTERNAL, err.Error())
		payload, _ = proto.Marshal(me.Proto())
	}
	out := replyMetadata(md, legacy)
	status := strconv.Itoa(int(me.Code()))
	out[MeshStatusKey] = status
	if legacy {
		out[GrpcStatusKey] = status
	}
	return mesh.Message{Metadata: out, Payload: payload}
}

// decode unmarshals payload into a fresh message of type M, obtained from
// the type's zero value, so M is a pointer to a generated message type. what
// names the payload in the error, such as "request" or "reply".
func decode[M proto.Message](what string, payload []byte) (M, error) {
	var zero M
	m := zero.ProtoReflect().New().Interface().(M)
	if err := proto.Unmarshal(payload, m); err != nil {
		return zero, fmt.Errorf("%s does not decode as %s: %w", what, m.ProtoReflect().Descriptor().FullName(), err)
	}
	return m, nil
}

// replyMetadata returns a copy of md without the keys the package sets, and
// with Mesh-Content-Type set, and Content-Type too when legacy is true.
func replyMetadata(md map[string]string, legacy bool) map[string]string {
	out := withoutWireKeys(md)
	out[MeshContentTypeKey] = ContentTypeProtobuf
	if legacy {
		out[ContentTypeKey] = ContentTypeProtobuf
	}
	return out
}

// consumerGroupMetadata is the metadata of a generated Endpoint or
// Subscriber: consumer_group when the method sets one, and nothing
// otherwise.
func consumerGroupMetadata(group string) map[string]string {
	if group == "" {
		return nil
	}
	return map[string]string{mesh.ConsumerGroupKey: group}
}
