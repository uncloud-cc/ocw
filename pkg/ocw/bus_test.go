package ocw

import (
	"testing"
	"time"
)

func TestEventBus_Subscribe(t *testing.T) {
	tests := []struct {
		name    string
		bufSize int
	}{
		{
			name:    "buffered channel",
			bufSize: 10,
		},
		{
			name:    "unbuffered channel",
			bufSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := NewEventBus()
			ch := bus.Subscribe(tt.bufSize)
			if ch == nil {
				t.Fatal("Subscribe() returned nil channel")
			}
			bus.Close()
		})
	}
}

func TestEventBus_Event(t *testing.T) {
	tests := []struct {
		name        string
		secrets     []string
		showSecrets bool
		event       Event
		check       func(t *testing.T, ev IngestedEvent)
	}{
		{
			name:        "log event reaches subscriber",
			secrets:     []string{},
			showSecrets: true,
			event:       &LogInfo{Message: "hello", Fields: map[string]any{}},
			check: func(t *testing.T, ev IngestedEvent) {
				if ev.Type != EventLogInfo {
					t.Errorf("expected type %q, got %q", EventLogInfo, ev.Type)
				}
				log, ok := ev.Event.(*LogInfo)
				if !ok {
					t.Fatalf("expected *LogInfo, got %T", ev.Event)
				}
				if log.Message != "hello" {
					t.Errorf("expected message %q, got %q", "hello", log.Message)
				}
			},
		},
		{
			name:        "secrets are masked when showSecrets=false",
			secrets:     []string{"secret123"},
			showSecrets: false,
			event:       &LogInfo{Message: "token=secret123", Fields: map[string]any{}},
			check: func(t *testing.T, ev IngestedEvent) {
				log, ok := ev.Event.(*LogInfo)
				if !ok {
					t.Fatalf("expected *LogInfo, got %T", ev.Event)
				}
				if log.Message != "token=[secret]" {
					t.Errorf("expected masked message, got %q", log.Message)
				}
			},
		},
		{
			name:        "secrets are not masked when showSecrets=true",
			secrets:     []string{"secret123"},
			showSecrets: true,
			event:       &LogInfo{Message: "token=secret123", Fields: map[string]any{}},
			check: func(t *testing.T, ev IngestedEvent) {
				log, ok := ev.Event.(*LogInfo)
				if !ok {
					t.Fatalf("expected *LogInfo, got %T", ev.Event)
				}
				if log.Message != "token=secret123" {
					t.Errorf("expected unmasked message, got %q", log.Message)
				}
			},
		},
		{
			name:        "metadata is populated",
			secrets:     []string{},
			showSecrets: true,
			event:       &LogDebug{Message: "debug msg", Fields: map[string]any{}},
			check: func(t *testing.T, ev IngestedEvent) {
				if ev.Type != EventLogDebug {
					t.Errorf("expected type %q, got %q", EventLogDebug, ev.Type)
				}
				if ev.Timestanmp == "" {
					t.Error("expected timestamp to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := NewEventBus()
			bus.SetSecrets(tt.showSecrets, tt.secrets)
			ch := bus.Subscribe(1)

			bus.Event(tt.event)

			select {
			case ev := <-ch:
				tt.check(t, ev)
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for event")
			}

			bus.Close()
		})
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	ch1 := bus.Subscribe(1)
	ch2 := bus.Subscribe(1)

	bus.Event(&LogInfo{Message: "broadcast", Fields: map[string]any{}})

	for i, ch := range []<-chan IngestedEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			log, ok := ev.Event.(*LogInfo)
			if !ok {
				t.Fatalf("subscriber %d: expected *LogInfo, got %T", i+1, ev.Event)
			}
			if log.Message != "broadcast" {
				t.Errorf("subscriber %d: expected message %q, got %q", i+1, "broadcast", log.Message)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i+1)
		}
	}

	bus.Close()
}

func TestEventBus_Close(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe(1)

	bus.Close()

	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed")
	}
}

func TestEventBus_ConvenienceMethods(t *testing.T) {
	tests := []struct {
		name     string
		emit     func(*EventBus)
		expected string
	}{
		{
			name:     "Debug",
			emit:     func(b *EventBus) { b.Debug("debug msg") },
			expected: EventLogDebug,
		},
		{
			name:     "Info",
			emit:     func(b *EventBus) { b.Info("info msg") },
			expected: EventLogInfo,
		},
		{
			name:     "Warn",
			emit:     func(b *EventBus) { b.Warn("warn msg") },
			expected: EventLogWarn,
		},
		{
			name:     "Error",
			emit:     func(b *EventBus) { b.Error("error msg") },
			expected: EventLogError,
		},
		{
			name:     "DebugWithData",
			emit:     func(b *EventBus) { b.DebugWithData("debug data", map[string]any{"key": "val"}) },
			expected: EventLogDebug,
		},
		{
			name:     "InfoWithData",
			emit:     func(b *EventBus) { b.InfoWithData("info data", map[string]any{"key": "val"}) },
			expected: EventLogInfo,
		},
		{
			name:     "WarnWithData",
			emit:     func(b *EventBus) { b.WarnWithData("warn data", map[string]any{"key": "val"}) },
			expected: EventLogWarn,
		},
		{
			name:     "ErrorWithData",
			emit:     func(b *EventBus) { b.ErrorWithData("error data", map[string]any{"key": "val"}) },
			expected: EventLogError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := NewEventBus()
			ch := bus.Subscribe(1)

			tt.emit(bus)

			select {
			case ev := <-ch:
				if ev.Type != tt.expected {
					t.Errorf("expected type %q, got %q", tt.expected, ev.Type)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for event")
			}

			bus.Close()
		})
	}
}
