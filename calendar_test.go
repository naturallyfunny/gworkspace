package gworkspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"golang.org/x/oauth2"
	calendarv3 "google.golang.org/api/calendar/v3"
)

func TestCalendarNotConnectedPropagates(t *testing.T) {
	cal, err := NewCalendar(NewClient(&fakeTokenStore{err: ErrNotConnected}, &oauth2.Config{Scopes: CalendarRequiredScopes}))
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"GetEvents", func() error { _, err := cal.GetEvents(ctx, "owner", EventQuery{}); return err }},
		{"AddEvent", func() error { _, err := cal.AddEvent(ctx, "owner", EventInput{}); return err }},
	}
	for _, tt := range tests {
		if err := tt.call(); !errors.Is(err, ErrNotConnected) {
			t.Errorf("%s err = %v, want ErrNotConnected", tt.name, err)
		}
	}
}

func TestNewCalendarMissingScopes(t *testing.T) {
	_, err := NewCalendar(NewClient(&fakeTokenStore{}, &oauth2.Config{}))
	if err == nil {
		t.Error("NewCalendar with empty scopes should return error")
	}
}

func TestEventFrom(t *testing.T) {
	e := &calendarv3.Event{
		Id:          "evt1",
		Summary:     "Standup",
		Description: "Daily sync",
		Location:    "Room A",
		Start:       &calendarv3.EventDateTime{DateTime: "2026-06-29T09:00:00Z"},
		End:         &calendarv3.EventDateTime{DateTime: "2026-06-29T09:30:00Z"},
		Attendees: []*calendarv3.EventAttendee{
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
	e := &calendarv3.Event{
		Id:    "evt2",
		Start: &calendarv3.EventDateTime{Date: "2026-06-29"},
		End:   &calendarv3.EventDateTime{Date: "2026-06-30"},
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
