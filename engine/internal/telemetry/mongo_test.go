package telemetry

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// newRecordingMonitor installs a recording tracer provider and returns a
// monitor wired to it. The monitor captures the provider at construction, so
// the provider has to be global before NewMongoMonitor is called — the same
// ordering main() depends on.
func newRecordingMonitor(t *testing.T) (*event.CommandMonitor, *tracetest.SpanRecorder) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))

	return NewMongoMonitor(), recorder
}

// rawCommand builds a wire command document. MongoDB puts the collection in
// the first element, keyed by the command name.
func rawCommand(t *testing.T, doc bson.D) bson.Raw {
	t.Helper()

	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}

	return raw
}

func attrsOf(span sdktrace.ReadOnlySpan) map[string]string {
	attrs := make(map[string]string)
	for _, attr := range span.Attributes() {
		attrs[string(attr.Key)] = attr.Value.Emit()
	}

	return attrs
}

func TestMongoMonitorRecordsSucceededCommand(t *testing.T) {
	monitor, recorder := newRecordingMonitor(t)

	started := &event.CommandStartedEvent{
		CommandName:  "find",
		DatabaseName: "runtz",
		RequestID:    7,
		ConnectionID: "mongo-0:27017",
		Command: rawCommand(t, bson.D{
			{Key: "find", Value: "users"},
			{Key: "filter", Value: bson.D{{Key: "email", Value: "alice@example.com"}}},
		}),
	}

	monitor.Started(context.Background(), started)
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName:  "find",
			DatabaseName: "runtz",
			RequestID:    7,
			ConnectionID: "mongo-0:27017",
		},
	})

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}

	span := spans[0]
	if got, want := span.Name(), "find runtz.users"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}
	if span.Status().Code == codes.Error {
		t.Errorf("successful command produced an error status: %v", span.Status())
	}

	attrs := attrsOf(span)
	for key, want := range map[string]string{
		"db.system":          "mongodb",
		"db.operation.name":  "find",
		"db.namespace":       "runtz",
		"db.collection.name": "users",
		"server.address":     "mongo-0:27017",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s = %q, want %q", key, got, want)
		}
	}
}

// The command document carries live application data — session tokens, API key
// hashes, e-mail addresses. None of it may reach the trace backend.
func TestMongoMonitorNeverRecordsCommandPayload(t *testing.T) {
	monitor, recorder := newRecordingMonitor(t)

	const secret = "session-token-that-must-not-leak"

	monitor.Started(context.Background(), &event.CommandStartedEvent{
		CommandName:  "insert",
		DatabaseName: "runtz",
		RequestID:    1,
		ConnectionID: "mongo-0:27017",
		Command: rawCommand(t, bson.D{
			{Key: "insert", Value: "sessions"},
			{Key: "documents", Value: bson.A{bson.D{{Key: "token", Value: secret}}}},
		}),
	})
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName:  "insert",
			DatabaseName: "runtz",
			RequestID:    1,
			ConnectionID: "mongo-0:27017",
		},
	})

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}

	for key, value := range attrsOf(spans[0]) {
		if value == secret {
			t.Errorf("attribute %s leaked the command payload", key)
		}
		// The attributes the OpenTelemetry conventions would use to carry a
		// full query. Neither should ever be set here.
		if key == "db.statement" || key == "db.query.text" {
			t.Errorf("attribute %s must never be set: got %q", key, value)
		}
	}
}

func TestMongoMonitorMarksFailedCommand(t *testing.T) {
	monitor, recorder := newRecordingMonitor(t)

	failure := errors.New("connection reset by peer")

	monitor.Started(context.Background(), &event.CommandStartedEvent{
		CommandName:  "update",
		DatabaseName: "runtz",
		RequestID:    99,
		ConnectionID: "mongo-0:27017",
		Command:      rawCommand(t, bson.D{{Key: "update", Value: "scans"}}),
	})
	monitor.Failed(context.Background(), &event.CommandFailedEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName:  "update",
			DatabaseName: "runtz",
			RequestID:    99,
			ConnectionID: "mongo-0:27017",
		},
		Failure: failure,
	})

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}

	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Errorf("status code = %v, want %v", span.Status().Code, codes.Error)
	}
	if span.Status().Description != failure.Error() {
		t.Errorf("status description = %q, want %q", span.Status().Description, failure.Error())
	}
	if len(span.Events()) == 0 {
		t.Error("failed command recorded no exception event")
	}
}

// The driver issues these constantly to keep connections and sessions healthy.
// Tracing them would bury the queries an operator actually wants to see.
func TestMongoMonitorSkipsDriverChatter(t *testing.T) {
	monitor, recorder := newRecordingMonitor(t)

	for _, command := range []string{"ping", "hello", "isMaster", "endSessions"} {
		monitor.Started(context.Background(), &event.CommandStartedEvent{
			CommandName:  command,
			DatabaseName: "admin",
			RequestID:    1,
			ConnectionID: "mongo-0:27017",
			Command:      rawCommand(t, bson.D{{Key: command, Value: int32(1)}}),
		})
		monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
			CommandFinishedEvent: event.CommandFinishedEvent{
				CommandName:  command,
				DatabaseName: "admin",
				RequestID:    1,
				ConnectionID: "mongo-0:27017",
			},
		})
	}

	if spans := recorder.Ended(); len(spans) != 0 {
		t.Errorf("got %d spans, want 0", len(spans))
	}
}

// getMore puts a cursor id where most commands put the collection name, so the
// span falls back to naming itself after the database alone.
func TestMongoMonitorHandlesNonStringCommandTarget(t *testing.T) {
	monitor, recorder := newRecordingMonitor(t)

	monitor.Started(context.Background(), &event.CommandStartedEvent{
		CommandName:  "getMore",
		DatabaseName: "runtz",
		RequestID:    3,
		ConnectionID: "mongo-0:27017",
		Command: rawCommand(t, bson.D{
			{Key: "getMore", Value: int64(4294967296)},
			{Key: "collection", Value: "scans"},
		}),
	})
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName:  "getMore",
			DatabaseName: "runtz",
			RequestID:    3,
			ConnectionID: "mongo-0:27017",
		},
	})

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if got, want := spans[0].Name(), "getMore runtz"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}
	if _, ok := attrsOf(spans[0])["db.collection.name"]; ok {
		t.Error("db.collection.name must be unset when the target is not a collection")
	}
}

// A command whose reply never arrives must not leave its span parked in the
// in-flight map forever.
func TestMongoMonitorDoesNotLeakUnfinishedSpans(t *testing.T) {
	monitor, recorder := newRecordingMonitor(t)

	monitor.Started(context.Background(), &event.CommandStartedEvent{
		CommandName:  "find",
		DatabaseName: "runtz",
		RequestID:    42,
		ConnectionID: "mongo-0:27017",
		Command:      rawCommand(t, bson.D{{Key: "find", Value: "users"}}),
	})

	// Nothing ended yet: the span is still open, waiting for a reply.
	if spans := recorder.Ended(); len(spans) != 0 {
		t.Fatalf("got %d ended spans before the reply, want 0", len(spans))
	}

	// A reply for a different request must not close it.
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName:  "find",
			DatabaseName: "runtz",
			RequestID:    43,
			ConnectionID: "mongo-0:27017",
		},
	})
	if spans := recorder.Ended(); len(spans) != 0 {
		t.Errorf("a mismatched reply ended %d spans, want 0", len(spans))
	}

	// The matching reply closes it exactly once.
	finished := event.CommandFinishedEvent{
		CommandName:  "find",
		DatabaseName: "runtz",
		RequestID:    42,
		ConnectionID: "mongo-0:27017",
	}
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{CommandFinishedEvent: finished})
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{CommandFinishedEvent: finished})

	if spans := recorder.Ended(); len(spans) != 1 {
		t.Errorf("got %d ended spans, want 1", len(spans))
	}
}
