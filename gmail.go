package gworkspace

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	gmail "google.golang.org/api/gmail/v1"
)

// gmailUser is the Gmail API's alias for the authenticated user.
const gmailUser = "me"

// maxMessages caps how many messages a single list call returns. Hardcoded;
// pagination deferred (see CLAUDE.md Design Decisions).
const maxMessages = 10

// Message is an email message. Body is the decoded text/plain part when present,
// otherwise empty.
type Message struct {
	ID       string
	ThreadID string
	From     string
	To       string
	Subject  string
	Snippet  string
	Body     string
	Labels   []string // label IDs applied to the message
	Date     time.Time
}

// Label is a Gmail label. Type is "system" (built-in, e.g. INBOX) or "user".
type Label struct {
	ID   string
	Name string
	Type string
}

// ReadMessages returns messages matching the Gmail search query (e.g.
// "is:unread", "from:alice@example.com"). An empty query lists recent messages.
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) ReadMessages(ctx context.Context, owner, query string) ([]Message, error) {
	svc, err := c.gmailFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	call := svc.Users.Messages.List(gmailUser).MaxResults(maxMessages).Context(ctx)
	if query != "" {
		call = call.Q(query)
	}
	res, err := call.Do()
	if err != nil {
		return nil, wrapError("read messages", err)
	}
	return c.fetchMessages(ctx, svc, res.Messages)
}

// GetMessagesByLabel returns messages carrying the given label ID.
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) GetMessagesByLabel(ctx context.Context, owner, labelID string) ([]Message, error) {
	svc, err := c.gmailFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	res, err := svc.Users.Messages.List(gmailUser).
		LabelIds(labelID).
		MaxResults(maxMessages).
		Context(ctx).
		Do()
	if err != nil {
		return nil, wrapError("get messages by label", err)
	}
	return c.fetchMessages(ctx, svc, res.Messages)
}

// fetchMessages turns the id-only stubs from a list call into full messages.
// Gmail's list endpoint returns only IDs, so each message must be fetched.
func (c *Client) fetchMessages(ctx context.Context, svc *gmail.Service, stubs []*gmail.Message) ([]Message, error) {
	messages := make([]Message, 0, len(stubs))
	for _, stub := range stubs {
		full, err := svc.Users.Messages.Get(gmailUser, stub.Id).Format("full").Context(ctx).Do()
		if err != nil {
			return nil, wrapError("get message", err)
		}
		messages = append(messages, messageFrom(full))
	}
	return messages, nil
}

// SendEmail sends a plain-text email from the owner to a single recipient.
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) SendEmail(ctx context.Context, owner, to, subject, body string) error {
	svc, err := c.gmailFor(ctx, owner)
	if err != nil {
		return err
	}
	raw := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s", to, subject, body)
	msg := &gmail.Message{Raw: base64.URLEncoding.EncodeToString([]byte(raw))}
	if _, err := svc.Users.Messages.Send(gmailUser, msg).Context(ctx).Do(); err != nil {
		return wrapError("send email", err)
	}
	return nil
}

// GetLabels returns all labels in the owner's mailbox.
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) GetLabels(ctx context.Context, owner string) ([]Label, error) {
	svc, err := c.gmailFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	res, err := svc.Users.Labels.List(gmailUser).Context(ctx).Do()
	if err != nil {
		return nil, wrapError("get labels", err)
	}
	labels := make([]Label, 0, len(res.Labels))
	for _, l := range res.Labels {
		labels = append(labels, labelFrom(l))
	}
	return labels, nil
}

// CreateLabel creates a user label and returns it.
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) CreateLabel(ctx context.Context, owner, name string) (Label, error) {
	svc, err := c.gmailFor(ctx, owner)
	if err != nil {
		return Label{}, err
	}
	created, err := svc.Users.Labels.Create(gmailUser, &gmail.Label{Name: name}).Context(ctx).Do()
	if err != nil {
		return Label{}, wrapError("create label", err)
	}
	return labelFrom(created), nil
}

// ApplyLabel adds the label to the message.
// Returns ErrNotConnected if the owner has not connected.
func (c *Client) ApplyLabel(ctx context.Context, owner, messageID, labelID string) error {
	svc, err := c.gmailFor(ctx, owner)
	if err != nil {
		return err
	}
	req := &gmail.ModifyMessageRequest{AddLabelIds: []string{labelID}}
	if _, err := svc.Users.Messages.Modify(gmailUser, messageID, req).Context(ctx).Do(); err != nil {
		return wrapError("apply label", err)
	}
	return nil
}

func labelFrom(l *gmail.Label) Label {
	return Label{ID: l.Id, Name: l.Name, Type: l.Type}
}

func messageFrom(m *gmail.Message) Message {
	msg := Message{
		ID:       m.Id,
		ThreadID: m.ThreadId,
		Snippet:  m.Snippet,
		Labels:   m.LabelIds,
	}
	// InternalDate is the message's epoch milliseconds — more reliable than the
	// Date header, which is sender-supplied and may be malformed.
	if m.InternalDate != 0 {
		msg.Date = time.UnixMilli(m.InternalDate)
	}
	if m.Payload != nil {
		msg.From = header(m.Payload.Headers, "From")
		msg.To = header(m.Payload.Headers, "To")
		msg.Subject = header(m.Payload.Headers, "Subject")
		msg.Body = bodyText(m.Payload)
	}
	return msg
}

// header returns the value of the named header (case-insensitive), or "".
func header(headers []*gmail.MessagePartHeader, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// bodyText walks a message part tree and returns the first text/plain body it
// finds, base64url-decoded. Falls back to the part's own body when there are no
// sub-parts.
func bodyText(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}
	if strings.HasPrefix(part.MimeType, "text/plain") && part.Body != nil && part.Body.Data != "" {
		return decodeBody(part.Body.Data)
	}
	for _, p := range part.Parts {
		if text := bodyText(p); text != "" {
			return text
		}
	}
	if len(part.Parts) == 0 && part.Body != nil && part.Body.Data != "" {
		return decodeBody(part.Body.Data)
	}
	return ""
}

// decodeBody decodes Gmail's base64url body data, tolerating both padded and
// unpadded encodings.
func decodeBody(data string) string {
	if b, err := base64.URLEncoding.DecodeString(data); err == nil {
		return string(b)
	}
	if b, err := base64.RawURLEncoding.DecodeString(data); err == nil {
		return string(b)
	}
	return ""
}
