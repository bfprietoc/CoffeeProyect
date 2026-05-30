package notification

// EmailSender sends transactional emails.
// A nil EmailSender is valid and silently drops all sends.
type EmailSender interface {
	Send(to, subject, body string) error
}
