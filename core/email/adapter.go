package email

import "context"

// Adapter wraps a Provider so it can be used wherever a plain SendEmail(ctx, to,
// subject, body) interface is required — notably subscription.EmailSender and the
// breach notification workflow.
type Adapter struct {
	provider Provider
	fromAddr string
}

// NewAdapter wraps provider. fromAddr is the RFC 5321 "From:" address for all
// outbound messages.
func NewAdapter(provider Provider, fromAddr string) *Adapter {
	return &Adapter{provider: provider, fromAddr: fromAddr}
}

// SendEmail satisfies the subscription.EmailSender interface.
func (a *Adapter) SendEmail(ctx context.Context, to, subject, body string) error {
	_, err := a.provider.Send(ctx, Message{
		To:       []string{to},
		From:     a.fromAddr,
		Subject:  subject,
		TextBody: body,
	})
	return err
}
