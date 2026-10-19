// Package protomesh is the Go library of the Proto Service Mesh API. It carries
// protobuf messages over any transport that implements the Service Mesh API
// contract in github.com/Paymentbox-com/service-mesh-go/mesh.
//
// The package holds the non-generated types of the specification: the
// TransportRouter, the Registry, the RPCRuntime, and MeshError, plus the
// generic helpers that generated code calls: NewEndpoint, NewSubscriber,
// Call, and Publish. Generated code lives in the definitions project and
// refers to the two process singletons, DefaultTransportRouter and
// DefaultRegistry, which the application configures at boot.
//
// Every message the package sends carries metadata Mesh-Content-Type set to
// application/x-protobuf. A reply that carries a MeshError also carries
// Mesh-Status, the google.rpc.Code as a decimal string, and its payload is
// the encoded google.rpc.Status.
//
// Content-Type and Grpc-Status are the deprecated forms of those keys, which
// older versions of the package send and read. A reply to a request that
// carries Content-Type without Mesh-Content-Type also carries Content-Type
// and, for a MeshError, Grpc-Status. Call reads Grpc-Status when a reply has
// no Mesh-Status. The first message on a Target that carries a deprecated
// key without its replacement logs a warning through slog.Default.
package protomesh

// Metadata keys the package writes and reads.
const (
	// MeshContentTypeKey is the metadata key every sent message carries.
	MeshContentTypeKey = "Mesh-Content-Type"

	// ContentTypeProtobuf is the value of MeshContentTypeKey on every sent
	// message.
	ContentTypeProtobuf = "application/x-protobuf"

	// MeshStatusKey marks a reply whose payload is an encoded
	// google.rpc.Status. Its value is the code as a decimal integer string.
	MeshStatusKey = "Mesh-Status"

	// ContentTypeKey is the key older versions send in place of
	// MeshContentTypeKey.
	//
	// Deprecated: Use MeshContentTypeKey.
	ContentTypeKey = "Content-Type"

	// GrpcStatusKey is the key older versions send in place of MeshStatusKey.
	//
	// Deprecated: Use MeshStatusKey.
	GrpcStatusKey = "Grpc-Status"

	// OptionPrefix starts the keys of an outgoing metadata map that are
	// transport options. Call and Publish pass each such key, with the prefix
	// removed, to the transport's Request or Publish options, and send the
	// other keys as message metadata. Reply metadata never carries such keys.
	// The match is exact and case-sensitive.
	OptionPrefix = "Mesh-Option-"

	// TransportKey is the Target metadata key naming the transport a Target
	// is served over. The TransportRouter resolves the value.
	TransportKey = "transport"
)
