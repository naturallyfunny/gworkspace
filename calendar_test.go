package gworkspace

import (
	"reflect"
	"testing"
	"time"

	calendar "google.golang.org/api/calendar/v3"
)

func TestEventFrom(t *testing.T) {
	e := &calendar.Event{
		Id:          "evt1",
		Summary:     "Standup",
		Description: "Daily sync",
		Location:    "Room A",
		Start:       &calendar.EventDateTime{DateTime: "2026-06-29T09:00:00Z"},
		End:         &calendar.EventDateTime{DateTime: "2026-06-29T09:30:00Z"},
		Attendees: []*calendar.EventAttendee{
			{Email: "a@example.com"},
			{Email: "b@example.com"},
		},
		HtmlLink: "https://cal.example/evt1",
	}

	want := Event{
		ID:          "evt1",
		Summary:     "Standup",
		Description: "Daily sync",
		Location:    "Room A",
		Start:       time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC),
		End:         time.Date(2026, 6, 29, 9, 30, 0, 0, time.UTC),
		Attendees:   []string{"a@example.com", "b@example.com"},
		HTMLLink:    "https://cal.example/evt1",
	}

	if got := eventFrom(e); !reflect.DeepEqual(got, want) {
		t.Errorf("eventFrom() = %+v, want %+v", got, want)
	}
}

func TestEventFromAllDay(t *testing.T) {
	e := &calendar.Event{
		Id:    "evt2",
		Start: &calendar.EventDateTime{Date: "2026-06-29"},
		End:   &calendar.EventDateTime{Date: "2026-06-30"},
	}
	got := eventFrom(e)
	want := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	if !got.Start.Equal(want) {
		t.Errorf("all-day Start = %v, want %v", got.Start, want)
	}
	if len(got.Attendees) != 0 {
		t.Errorf("Attendees = %v, want empty", got.Attendees)
	}
}
