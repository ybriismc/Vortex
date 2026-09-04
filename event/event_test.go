package event

import (
	"io"
	"log/slog"
	"testing"
)

type sample struct {
	Cancelling

	Value string
}

func testBus() *Bus {
	return NewBus(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHandlersRunInPriorityOrder(t *testing.T) {
	bus := testBus()
	var order []string

	Subscribe(bus, func(*sample) { order = append(order, "monitor") }, Monitor)
	Subscribe(bus, func(*sample) { order = append(order, "normal") }, Normal)
	Subscribe(bus, func(*sample) { order = append(order, "lowest") }, Lowest)
	Subscribe(bus, func(*sample) { order = append(order, "normal-second") }, Normal)

	Call(bus, &sample{})

	expected := []string{"lowest", "normal", "normal-second", "monitor"}
	if len(order) != len(expected) {
		t.Fatalf("ran %v handlers, expected %v", len(order), len(expected))
	}

	for i, name := range expected {
		if order[i] != name {
			t.Fatalf("handler %d was %q, expected %q", i, order[i], name)
		}
	}
}

func TestHandlersSeeCancellationAndChanges(t *testing.T) {
	bus := testBus()
	Subscribe(bus, func(e *sample) {
		e.Value = "changed"
		e.Cancel()
	}, Normal)

	cancelled := false
	Subscribe(bus, func(e *sample) { cancelled = e.Cancelled() }, Monitor)

	e := Call(bus, &sample{Value: "original"})
	if !e.Cancelled() {
		t.Fatal("expected the event to be cancelled")
	}

	if e.Value != "changed" {
		t.Fatalf("value is %q, expected \"changed\"", e.Value)
	}

	if !cancelled {
		t.Fatal("expected the monitor handler to see the cancellation")
	}
}

func TestPanicInHandlerDoesNotStopTheOthers(t *testing.T) {
	bus := testBus()
	reached := false

	Subscribe(bus, func(*sample) { panic("boom") }, Normal)
	Subscribe(bus, func(*sample) { reached = true }, High)

	Call(bus, &sample{})
	if !reached {
		t.Fatal("expected the second handler to run after the panic")
	}
}

func TestSubscribedReportsHandlers(t *testing.T) {
	bus := testBus()
	if Subscribed[sample](bus) {
		t.Fatal("expected no handlers")
	}

	Subscribe(bus, func(*sample) {}, Normal)
	if !Subscribed[sample](bus) {
		t.Fatal("expected the event to have a handler")
	}

	if Subscribed[sample](nil) {
		t.Fatal("expected a nil bus to report no handlers")
	}
}

func TestForSharesTheHandlers(t *testing.T) {
	bus := testBus()
	called := false
	Subscribe(bus.For("plugin"), func(*sample) { called = true }, Normal)

	Call(bus, &sample{})
	if !called {
		t.Fatal("expected the handler subscribed through the view to run")
	}
}
