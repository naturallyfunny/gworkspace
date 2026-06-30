package gworkspace

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"golang.org/x/oauth2"
	peoplev1 "google.golang.org/api/people/v1"
)

func TestContactsNotConnectedPropagates(t *testing.T) {
	con, err := NewContacts(NewClient(&fakeTokenStore{err: ErrNotConnected}, &oauth2.Config{Scopes: ContactsRequiredScopes}))
	if err != nil {
		t.Fatalf("NewContacts: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"GetContacts", func() error { _, err := con.GetContacts(ctx, "owner", ContactQuery{}); return err }},
		{"AddContact", func() error { _, err := con.AddContact(ctx, "owner", ContactInput{}); return err }},
	}
	for _, tt := range tests {
		if err := tt.call(); !errors.Is(err, ErrNotConnected) {
			t.Errorf("%s err = %v, want ErrNotConnected", tt.name, err)
		}
	}
}

func TestNewContactsMissingScopes(t *testing.T) {
	_, err := NewContacts(NewClient(&fakeTokenStore{}, &oauth2.Config{}))
	if err == nil {
		t.Error("NewContacts with empty scopes should return error")
	}
}

func TestContactFrom(t *testing.T) {
	p := &peoplev1.Person{
		ResourceName: "people/c123",
		Names:        []*peoplev1.Name{{DisplayName: "Alice Smith"}},
		EmailAddresses: []*peoplev1.EmailAddress{
			{Value: "alice@example.com"},
			{Value: "a.smith@work.com"},
		},
		PhoneNumbers: []*peoplev1.PhoneNumber{{Value: "+1234567890"}},
	}

	want := Contact{
		ResourceName: "people/c123",
		Name:         "Alice Smith",
		Emails:       []string{"alice@example.com", "a.smith@work.com"},
		Phones:       []string{"+1234567890"},
	}

	if got := contactFrom(p); !reflect.DeepEqual(got, want) {
		t.Errorf("contactFrom() = %+v, want %+v", got, want)
	}
}

func TestContactFromEmpty(t *testing.T) {
	got := contactFrom(&peoplev1.Person{ResourceName: "people/c0"})
	if got.Name != "" || len(got.Emails) != 0 || len(got.Phones) != 0 {
		t.Errorf("contactFrom(empty) = %+v", got)
	}
}
