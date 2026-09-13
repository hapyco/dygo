package notifications

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/hapyco/dygo/pkg/dygo"
)

// Mailer delivers one plain-text notification email.
type Mailer interface {
	Send(context.Context, string, string, string) error
}

// SMTPMailer delivers email through an SMTP server with optional plain authentication.
type SMTPMailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	SendMail func(string, smtp.Auth, string, []string, []byte) error
}

// Send delivers one message through SMTP.
func (m SMTPMailer) Send(ctx context.Context, recipient string, subject string, body string) error {
	from, err := mail.ParseAddress(strings.TrimSpace(m.From))
	if err != nil {
		return fmt.Errorf("SMTP from address is invalid: %w", err)
	}
	to, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil {
		return fmt.Errorf("notification recipient email is invalid: %w", err)
	}
	host := strings.TrimSpace(m.Host)
	if host == "" || m.Port < 1 {
		return fmt.Errorf("SMTP is not configured")
	}
	var auth smtp.Auth
	if strings.TrimSpace(m.Username) != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, host)
	}
	message := []byte("From: " + cleanHeader(m.From) + "\r\n" +
		"To: " + cleanHeader(recipient) + "\r\n" +
		"Subject: " + cleanHeader(subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body + "\r\n")
	address := host + ":" + strconv.Itoa(m.Port)
	if m.SendMail != nil {
		return m.SendMail(address, auth, from.Address, []string{to.Address}, message)
	}
	return sendSMTP(ctx, address, host, auth, from.Address, to.Address, message)
}

// UnavailableMailer reports missing SMTP configuration to the durable Job retry path.
type UnavailableMailer struct{}

// Send reports that SMTP must be configured before email can be delivered.
func (UnavailableMailer) Send(context.Context, string, string, string) error {
	return fmt.Errorf("SMTP is not configured")
}

// EmailJob returns the system Job handler for notification email delivery.
func EmailJob(mailer Mailer, now func() time.Time) dygo.JobFunc {
	if mailer == nil {
		mailer = UnavailableMailer{}
	}
	if now == nil {
		now = time.Now
	}
	return func(ctx context.Context, execution dygo.JobExecution) error {
		var payload struct {
			NotificationID int64 `json:"notification-id"`
		}
		if err := json.Unmarshal(execution.Payload, &payload); err != nil || payload.NotificationID <= 0 {
			return fmt.Errorf("notification email payload must contain a positive notification-id")
		}
		record, err := execution.Records.Get(ctx, "core", "notification", payload.NotificationID)
		if err != nil {
			return err
		}
		if emailedAt, _ := record["emailed-at"].(string); strings.TrimSpace(emailedAt) != "" {
			return nil
		}
		if requested, _ := record["send-email"].(bool); !requested {
			return nil
		}
		recipient, _ := record["recipient"].(string)
		title, _ := record["title"].(string)
		message, _ := record["message"].(string)
		if err := mailer.Send(ctx, recipient, title, message); err != nil {
			return err
		}
		encoded, _ := json.Marshal(now().UTC().Format(time.RFC3339))
		_, err = execution.Records.Update(ctx, "core", "notification", payload.NotificationID, dygo.RecordInput{"emailed-at": encoded})
		return err
	}
}

func cleanHeader(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(value))
}

func sendSMTP(ctx context.Context, address string, host string, auth smtp.Auth, from string, to string, message []byte) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
