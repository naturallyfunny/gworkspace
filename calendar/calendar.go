// Package calendar provides Google Calendar access for a Workspace owner.
// Construct a Service with a gworkspace.Connector (typically *gworkspace.Client)
// and call its methods; OAuth tokens are resolved per request.
package calendar

import (
	"context"
	"time"

	calendarv3 "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"go.naturallyfunny.dev/gworkspace"
)

const defaultCalendarID = "primary"

// Service provides Google Calendar access for a Workspace owner.
type Service struct {
	conn gworkspace.Connector
}

// New builds a Service that uses conn to obtain per-owner OAuth tokens.
func New(conn gworkspace.Connector) *Service {
	return &Service{conn: conn}
}

// Event is a calendar event. Start and End are the event's instants; for all-day
// events they are the date at midnight UTC.
type Event struct {
	ID          string
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	Attendees   []string // attendee email addresses
	HTMLLink    string
}

// EventQuery filters a GetEvents call. The zero value lists upcoming events on
// the primary calendar. CalendarID defaults to "primary"; Query is a free-text
// search. TimeMin defaults to now (so the zero query is forward-looking); set it
// to a past time to include history. TimeMax is open unless set. Limit sets the
// maximum results returned; zero or negative applies no cap (Google Calendar
// API default applies).
type EventQuery struct {
	CalendarID string
	Query      string
	TimeMin    time.Time
	TimeMax    time.Time
	Limit      int
}

// EventInput describes an event to create. CalendarID defaults to "primary".
// Guests are added as attendees by email address.
type EventInput struct {
	CalendarID  string
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	Guests      []string
}

// GetEvents returns events for the owner matching q, ordered by start time.
// Returns gworkspace.ErrNotConnected if the owner has not connected.
func (s *Service) GetEvents(ctx context.Context, owner string, q EventQuery) ([]Event, error) {
	svc, err := s.calendarFor(ctx, owner)
	if err != nil {
		return nil, err
	}

	calendarID := q.CalendarID
	if calendarID == "" {
		calendarID = defaultCalendarID
	}

	call := svc.Events.List(calendarID).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx)
	if q.Query != "" {
		call = call.Q(q.Query)
	}
	// OrderBy("startTime") with no lower bound would return events from the
	// earliest available date; default TimeMin to now so the zero query is
	// forward-looking (matching Google's own quickstart).
	timeMin := q.TimeMin
	if timeMin.IsZero() {
		timeMin = time.Now()
	}
	call = call.TimeMin(timeMin.Format(time.RFC3339))
	if !q.TimeMax.IsZero() {
		call = call.TimeMax(q.TimeMax.Format(time.RFC3339))
	}
	if q.Limit > 0 {
		call = call.MaxResults(int64(q.Limit))
	}

	res, err := call.Do()
	if err != nil {
		return nil, gworkspace.WrapError("get events", err)
	}
	events := make([]Event, 0, len(res.Items))
	for _, item := range res.Items {
		events = append(events, eventFrom(item))
	}
	return events, nil
}

// AddEvent creates an event for the owner and returns the created event.
// Returns gworkspace.ErrNotConnected if the owner has not connected.
func (s *Service) AddEvent(ctx context.Context, owner string, in EventInput) (Event, error) {
	svc, err := s.calendarFor(ctx, owner)
	if err != nil {
		return Event{}, err
	}

	calendarID := in.CalendarID
	if calendarID == "" {
		calendarID = defaultCalendarID
	}

	attendees := make([]*calendarv3.EventAttendee, 0, len(in.Guests))
	for _, g := range in.Guests {
		attendees = append(attendees, &calendarv3.EventAttendee{Email: g})
	}

	event := &calendarv3.Event{
		Summary:     in.Summary,
		Description: in.Description,
		Location:    in.Location,
		Start:       &calendarv3.EventDateTime{DateTime: in.Start.Format(time.RFC3339)},
		End:         &calendarv3.EventDateTime{DateTime: in.End.Format(time.RFC3339)},
		Attendees:   attendees,
	}

	created, err := svc.Events.Insert(calendarID, event).Context(ctx).Do()
	if err != nil {
		return Event{}, gworkspace.WrapError("add event", err)
	}
	return eventFrom(created), nil
}

func (s *Service) calendarFor(ctx context.Context, owner string) (*calendarv3.Service, error) {
	ts, err := s.conn.TokenSourceFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	return calendarv3.NewService(ctx, option.WithTokenSource(ts))
}

func eventFrom(e *calendarv3.Event) Event {
	attendees := make([]string, 0, len(e.Attendees))
	for _, a := range e.Attendees {
		attendees = append(attendees, a.Email)
	}
	return Event{
		ID:          e.Id,
		Summary:     e.Summary,
		Description: e.Description,
		Location:    e.Location,
		Start:       parseEventTime(e.Start),
		End:         parseEventTime(e.End),
		Attendees:   attendees,
		HTMLLink:    e.HtmlLink,
	}
}

// parseEventTime reads an EventDateTime, which carries either DateTime (RFC3339,
// for timed events) or Date (yyyy-mm-dd, for all-day events). Returns the zero
// time when neither parses.
func parseEventTime(dt *calendarv3.EventDateTime) time.Time {
	if dt == nil {
		return time.Time{}
	}
	if dt.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, dt.DateTime); err == nil {
			return t
		}
	}
	if dt.Date != "" {
		if t, err := time.Parse("2006-01-02", dt.Date); err == nil {
			return t
		}
	}
	return time.Time{}
}

