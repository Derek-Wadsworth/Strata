package tracker

// EventType identifies a tracker event.
type EventType string

const (
	// EventVersionCreated is emitted when a new version is recorded.
	EventVersionCreated EventType = "version_created"
)

// Event is a notification for live subscribers (CLI, GUI, etc.).
type Event struct {
	Type       EventType
	Path       string
	VersionNum int
}

// Subscribe registers for tracker events. Call the returned unsubscribe function
// when finished; it closes the channel. Slow subscribers may miss events.
func (t *Tracker) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 16
	}
	ch := make(chan Event, buffer)

	t.mu.Lock()
	if t.subs == nil {
		t.subs = make(map[chan Event]struct{})
	}
	t.subs[ch] = struct{}{}
	t.mu.Unlock()

	unsub := func() {
		t.mu.Lock()
		if _, ok := t.subs[ch]; ok {
			delete(t.subs, ch)
			close(ch)
		}
		t.mu.Unlock()
	}
	return ch, unsub
}

func (t *Tracker) emit(ev Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for ch := range t.subs {
		select {
		case ch <- ev:
		default:
			// Drop if the subscriber is not keeping up.
		}
	}
}

func (t *Tracker) emitVersionCreated(path string, versionNum int) {
	t.emit(Event{
		Type:       EventVersionCreated,
		Path:       path,
		VersionNum: versionNum,
	})
}
