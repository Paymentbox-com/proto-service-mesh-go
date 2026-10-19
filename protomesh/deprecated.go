package protomesh

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/Paymentbox-com/service-mesh-go/mesh"
)

// warned holds a key and a Target for every deprecated key already logged.
var warned sync.Map

// legacyContentType reports whether md comes from an older version: it
// carries Content-Type and no Mesh-Content-Type. The first such message on t
// logs a warning.
func legacyContentType(t mesh.Target, md map[string]string) bool {
	return deprecated(t, md, ContentTypeKey, MeshContentTypeKey)
}

// legacyStatus reports whether reply metadata md carries Grpc-Status and no
// Mesh-Status. The first such reply on t logs a warning.
func legacyStatus(t mesh.Target, md map[string]string) bool {
	return deprecated(t, md, GrpcStatusKey, MeshStatusKey)
}

func deprecated(t mesh.Target, md map[string]string, old, replacement string) bool {
	if _, ok := md[old]; !ok {
		return false
	}
	if _, ok := md[replacement]; ok {
		return false
	}
	if _, seen := warned.LoadOrStore(old+"\x00"+strings.Join(t.Segments, "\x00"), true); !seen {
		slog.Default().Warn("protomesh: message carries a deprecated metadata key; the sender runs an older version",
			"key", old, "replacement", replacement, "target", t.Segments)
	}
	return true
}

// withoutWireKeys returns a copy of md without the keys the package sets.
func withoutWireKeys(md map[string]string) map[string]string {
	out := make(map[string]string, len(md)+2)
	for k, v := range md {
		switch k {
		case MeshContentTypeKey, MeshStatusKey, ContentTypeKey, GrpcStatusKey:
		default:
			out[k] = v
		}
	}
	return out
}
