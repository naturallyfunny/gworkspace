package gworkspace

import (
	"context"

	people "google.golang.org/api/people/v1"
)

// personFields selects which fields the People API returns and which it reads on
// create — the subset the Contact type exposes.
const personFields = "names,emailAddresses,phoneNumbers"

// maxContacts caps how many contacts GetContacts returns. Hardcoded; pagination
// deferred (see CLAUDE.md Design Decisions).
const maxContacts = 50

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
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) GetContacts(ctx context.Context, owner string) ([]Contact, error) {
	svc, err := c.peopleFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	res, err := svc.People.Connections.List("people/me").
		PersonFields(personFields).
		PageSize(maxContacts).
		Context(ctx).
		Do()
	if err != nil {
		return nil, wrapError("get contacts", err)
	}
	contacts := make([]Contact, 0, len(res.Connections))
	for _, p := range res.Connections {
		contacts = append(contacts, contactFrom(p))
	}
	return contacts, nil
}

// AddContact creates a contact for the owner and returns it.
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) AddContact(ctx context.Context, owner string, in ContactInput) (Contact, error) {
	svc, err := c.peopleFor(ctx, owner)
	if err != nil {
		return Contact{}, err
	}

	person := &people.Person{}
	if in.Name != "" {
		person.Names = []*people.Name{{GivenName: in.Name}}
	}
	for _, e := range in.Emails {
		person.EmailAddresses = append(person.EmailAddresses, &people.EmailAddress{Value: e})
	}
	for _, p := range in.Phones {
		person.PhoneNumbers = append(person.PhoneNumbers, &people.PhoneNumber{Value: p})
	}

	// Request personFields on create too: without it the People API returns the
	// person without computed fields like Names[0].DisplayName, so contactFrom
	// would yield an empty Name on the created contact.
	created, err := svc.People.CreateContact(person).PersonFields(personFields).Context(ctx).Do()
	if err != nil {
		return Contact{}, wrapError("add contact", err)
	}
	return contactFrom(created), nil
}

func contactFrom(p *people.Person) Contact {
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
