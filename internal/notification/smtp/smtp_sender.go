package smtp

import (
	"fmt"
	"net/smtp"
)

type Sender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewSender(host string, port int, username, password, from string) *Sender {
	return &Sender{host: host, port: port, username: username, password: password, from: from}
}

func (s *Sender) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, body,
	))
	return smtp.SendMail(addr, auth, s.from, []string{to}, msg)
}
