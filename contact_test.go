package gworkspace

import (
	"reflect"
	"testing"

	people "google.golang.org/api/people/v1"
)

func TestContactFrom(t *testing.T) {
	p := &people.Person{
		ResourceName: "people/c123",
		Names:        []*people.Name{{DisplayName: "Alice Smith"}},
		EmailAddresses: []*people.EmailAddress{
			{Value: "alice@example.com"},
			{Value: "a.smith@work.com"},
		},
		PhoneNumbers: []*people.PhoneNumber{{Value: "+1234567890"}},
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
	got := contactFrom(&people.Person{ResourceName: "people/c0"})
	if got.Name != "" || len(got.Emails) != 0 || len(got.Phones) != 0 {
		t.Errorf("contactFrom(empty) = %+v", got)
	}
}
