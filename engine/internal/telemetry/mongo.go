package telemetry

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// mongoTracerName identifies the spans this file produces.
const mongoTracerName = "github.com/runtz-dev/runtz/engine/internal/telemetry/mongo"

// driverChatter is the set of commands the driver issues to keep connections
// and sessions healthy. They fire constantly, carry no application meaning,
// and would bury the queries an operator actually wants to see.
var driverChatter = map[string]struct{}{
	"hello":                   {},
	"isMaster":                {},
	"ismaster":                {},
	"ping":                    {},
	"buildInfo":               {},
	"getLastError":            {},
	"endSessions":             {},
	"killCursors":             {},
	"saslStart":               {},
	"saslContinue":            {},
	"authenticate":            {},
	"getnonce":                {},
	"createIndexes":           {},
	"listIndexes":             {},
	"connectionStatus":        {},
	"getFreeMonitoringStatus": {},
}

// NewMongoMonitor returns a CommandMonitor that opens a span for every command
// the driver sends and closes it when the server answers.
//
// We hand-roll this instead of using contrib's otelmongo because otelmongo has
// only ever been released for mongo-driver v1, and the engine is on v2. The v2
// port exists upstream but carries no tag, and an untagged OpenTelemetry
// module is not something to put in the engine's dependency graph. Revisit
// when contrib tags
// go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo.
//
// The command document itself is never recorded. It holds live application
// data — session tokens, API key hashes, e-mail addresses — and a trace
// backend is not the place for any of it. Only the command name, the database
// and the collection go on the span.
func NewMongoMonitor() *event.CommandMonitor {
	tracer := otel.GetTracerProvider().Tracer(mongoTracerName)

	// A span lives between two callbacks, so it has to be parked somewhere in
	// between. The driver guarantees request ids are unique per connection,
	// which makes the pair a safe key for concurrent in-flight commands.
	var inFlight sync.Map // mongoSpanKey -> trace.Span

	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			if _, skip := driverChatter[evt.CommandName]; skip {
				return
			}

			attrs := []attribute.KeyValue{
				semconv.DBSystemMongoDB,
				semconv.DBOperationName(evt.CommandName),
			}
			if evt.DatabaseName != "" {
				attrs = append(attrs, semconv.DBNamespace(evt.DatabaseName))
			}
			if collection := collectionOf(evt); collection != "" {
				attrs = append(attrs, semconv.DBCollectionName(collection))
			}
			if evt.ConnectionID != "" {
				// "host:port" of the mongod that served the command, which is
				// what tells replicas apart once MongoDB is more than one pod.
				attrs = append(attrs, semconv.ServerAddress(evt.ConnectionID))
			}

			_, span := tracer.Start(ctx, spanName(evt),
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(attrs...),
			)

			// Nothing will be exported for a span that is not recording —
			// either telemetry is off entirely or the trace was sampled out —
			// so end it here rather than paying for a map entry per query and
			// a second callback to take it back out.
			if !span.IsRecording() {
				span.End()
				return
			}

			inFlight.Store(mongoSpanKey{connectionID: evt.ConnectionID, requestID: evt.RequestID}, span)
		},

		Succeeded: func(_ context.Context, evt *event.CommandSucceededEvent) {
			span, ok := takeSpan(&inFlight, evt.ConnectionID, evt.RequestID)
			if !ok {
				return
			}
			span.End()
		},

		Failed: func(_ context.Context, evt *event.CommandFailedEvent) {
			span, ok := takeSpan(&inFlight, evt.ConnectionID, evt.RequestID)
			if !ok {
				return
			}
			if evt.Failure != nil {
				span.RecordError(evt.Failure)
				span.SetStatus(codes.Error, evt.Failure.Error())
			} else {
				span.SetStatus(codes.Error, "mongo command failed")
			}
			span.End()
		},
	}
}

// mongoSpanKey identifies one in-flight command.
type mongoSpanKey struct {
	connectionID string
	requestID    int64
}

// takeSpan removes and returns the span opened for a command, so a span is
// never ended twice and nothing is left behind in the map.
func takeSpan(inFlight *sync.Map, connectionID string, requestID int64) (trace.Span, bool) {
	value, ok := inFlight.LoadAndDelete(mongoSpanKey{connectionID: connectionID, requestID: requestID})
	if !ok {
		return nil, false
	}

	span, ok := value.(trace.Span)

	return span, ok
}

// spanName follows the OpenTelemetry database convention of
// "<operation> <target>", which is what makes traces group by query shape
// rather than by individual call.
func spanName(evt *event.CommandStartedEvent) string {
	if collection := collectionOf(evt); collection != "" {
		return evt.CommandName + " " + evt.DatabaseName + "." + collection
	}
	if evt.DatabaseName != "" {
		return evt.CommandName + " " + evt.DatabaseName
	}

	return evt.CommandName
}

// collectionOf reads the collection out of the command document. MongoDB wire
// commands put it in the first element, keyed by the command name itself
// (`{"find": "users", ...}`), so this is a cheap lookup rather than a full
// unmarshal — and it returns empty for the commands that take a number there
// instead, like getMore.
func collectionOf(evt *event.CommandStartedEvent) string {
	value, err := evt.Command.LookupErr(evt.CommandName)
	if err != nil {
		return ""
	}

	collection, ok := value.StringValueOK()
	if !ok {
		return ""
	}

	return collection
}
