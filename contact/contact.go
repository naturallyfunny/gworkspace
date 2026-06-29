// Package contact provides Google Contacts access for a Workspace owner.
// Construct a Service with a gworkspace.Connector (typically *gworkspace.Client)
// and call its methods; OAuth tokens are resolved per request.
package contact

import (
	"context"

	peoplev1 "google.golang.org/api/people/v1"
	"google.golang.org/api/option"

	"go.naturallyfunny.dev/gworkspace"
)

const personFields = "names,emailAddresses,phoneNumbers"

// ContactQuery filters a GetContacts call. Limit sets the maximum results
// returned; zero or negative applies no cap (People API default applies).
type ContactQuery struct {
	Limit int
}

// Service provides Google Contacts access for a Workspace owner.
type Service struct {
	conn gworkspace.Connector
}

// New builds a Service that uses conn to obtain per-owner OAuth tokens.
func New(conn gworkspace.Connector) *Service {
	return &Service{conn: conn}
}

// Contact is a person in the owner's Google Contacts. ResourceName is the People
// API identifier (e.g. "people/c123").
type Contact struct {
	ResourceName string
	Name         string
	Emails       []string
	Phones       []string
}

// ContactInput describes a contact to create.
type ContactInput struct {
	Name   string
	Emails []string
	Phones []string
}

// GetContacts returns the owner's contacts.
// Returns gworkspace.ErrNotConnected if the owner has not connected.
func (s *Service) GetContacts(ctx context.Context, owner string, q ContactQuery) ([]Contact, error) {
	svc, err := s.peopleFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	call := svc.People.Connections.List("people/me").
		PersonFields(personFields).
		Context(ctx)
	if q.Limit > 0 {
		call = call.PageSize(int64(q.Limit))
	}
	res, err := call.Do()
	if err != nil {
		return nil, gworkspace.WrapError("get contacts", err)
	}
	contacts := make([]Contact, 0, len(res.Connections))
	for _, p := range res.Connections {
		contacts = append(contacts, contactFrom(p))
	}
	return contacts, nil
}

// AddContact creates a contact for the owner and returns it.
// Returns gworkspace.ErrNotConnected if the owner has not connected.
func (s *Service) AddContact(ctx context.Context, owner string, in ContactInput) (Contact, error) {
	svc, err := s.peopleFor(ctx, owner)
	if err != nil {
		return Contact{}, err
	}

	person := &peoplev1.Person{}
	if in.Name != "" {
		person.Names = []*peoplev1.Name{{GivenName: in.Name}}
	}
	for _, e := range in.Emails {
		person.EmailAddresses = append(person.EmailAddresses, &peoplev1.EmailAddress{Value: e})
	}
	for _, p := range in.Phones {
		person.PhoneNumbers = append(person.PhoneNumbers, &peoplev1.PhoneNumber{Value: p})
	}

	// Request personFields on create too: without it the People API returns the
	// person without computed fields like Names[0].DisplayName, so contactFrom
	// would yield an empty Name on the created contact.
	created, err := svc.People.CreateContact(person).PersonFields(personFields).Context(ctx).Do()
	if err != nil {
		return Contact{}, gworkspace.WrapError("add contact", err)
	}
	return contactFrom(created), nil
}

func (s *Service) peopleFor(ctx context.Context, owner string) (*peoplev1.Service, error) {
	ts, err := s.conn.TokenSourceFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	return peoplev1.NewService(ctx, option.WithTokenSource(ts))
}

func contactFrom(p *peoplev1.Person) Contact {
	contact := Contact{ResourceName: p.ResourceName}
	if len(p.Names) > 0 {
		contact.Name = p.Names[0].DisplayName
	}
	for _, e := range p.EmailAddresses {
		contact.Emails = append(contact.Emails, e.Value)
	}
	for _, ph := range p.PhoneNumbers {
		contact.Phones = append(contact.Phones, ph.Value)
	}
	return contact
}
