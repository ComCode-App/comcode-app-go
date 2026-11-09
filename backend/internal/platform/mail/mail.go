package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
)

// Message is an outbound email: a plain-text body and optional attachments.
type Message struct {
	To          []string
	Subject     string
	Body        string
	Attachments []Attachment
}

// Attachment is a file sent with a message (base64 encoded on the wire).
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Sender delivers email messages.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// sendTimeout bounds one delivery (dial, TLS, auth, DATA) when ctx has no earlier deadline.
const sendTimeout = 30 * time.Second

// TLS modes (SMTP_TLS).
const (
	TLSAuto     = "auto"     // implicit TLS on port 465, STARTTLS when the server offers it elsewhere
	TLSImplicit = "implicit" // TLS from the first byte (SMTPS)
	TLSStartTLS = "starttls" // plain connect, STARTTLS required
	TLSNone     = "none"     // plain text (MailHog in development)
)

// SMTPSender sends mail via SMTP (MailHog in development).
type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
	tlsMode  string
	rootCAs  *x509.CertPool // nil = system roots; tests inject their own
}

// NewSMTPSender builds a sender from config.
func NewSMTPSender(cfg config.SMTPConfig) *SMTPSender {
	mode := strings.ToLower(strings.TrimSpace(cfg.TLS))
	if mode == "" {
		mode = TLSAuto
	}
	return &SMTPSender{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
		fromName: cfg.FromName,
		tlsMode:  mode,
	}
}

// implicitTLS reports whether the connection must start with a TLS handshake.
func (s *SMTPSender) implicitTLS() bool {
	return s.tlsMode == TLSImplicit || (s.tlsMode == TLSAuto && s.port == 465)
}

// Send delivers a plain-text email.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if s == nil {
		return fmt.Errorf("mail: sender is nil")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail: recipient required")
	}
	from := s.from
	if from == "" {
		from = "noreply@localhost"
	}

	deadline := time.Now().Add(sendTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	dialer := &net.Dialer{Deadline: deadline}
	tlsConfig := &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12, RootCAs: s.rootCAs}

	var conn net.Conn
	var err error
	if s.implicitTLS() {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("mail: dial %s: %w", addr, err)
	}
	// Without this a server that never answers (e.g. plain text on an SMTPS port) blocks forever.
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mail: handshake: %w", err)
	}
	defer func() { _ = c.Close() }()

	if !s.implicitTLS() && s.tlsMode != TLSNone {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("mail: starttls: %w", err)
			}
		} else if s.tlsMode == TLSStartTLS {
			return fmt.Errorf("mail: server does not offer STARTTLS")
		}
	}
	if s.username != "" {
		// PlainAuth refuses to send credentials over an unencrypted non-localhost connection.
		if err := c.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	for _, to := range msg.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("mail: rcpt: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write([]byte(s.payload(from, msg))); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	// The server accepted the message once DATA closed with 250. A failed QUIT
	// (an error reply or a dropped connection) must not report the send as
	// failed, or the queue would retry and deliver a duplicate email.
	if err := c.Quit(); err != nil {
		slog.WarnContext(ctx, "mail: QUIT failed after message was accepted", "addr", addr, "error", err)
	}
	return nil
}

func (s *SMTPSender) payload(from string, msg Message) string {
	fromHeader := from
	if s.fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", s.fromName), from)
	}
	var b strings.Builder
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(msg.Subject)
	// Q-encode so non-ASCII names/subjects (Turkish) survive; CR/LF are stripped so a subject cannot add headers.
	fmt.Fprintf(&b, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	body := strings.ReplaceAll(strings.ReplaceAll(msg.Body, "\r\n", "\n"), "\n", "\r\n")
	if len(msg.Attachments) == 0 {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(body)
		return b.String()
	}
	boundary := newBoundary()
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")
	for _, a := range msg.Attachments {
		ctype := a.ContentType
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		name := strings.NewReplacer("\r", "", "\n", "", "\"", "").Replace(a.Filename)
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		fmt.Fprintf(&b, "Content-Type: %s; name=\"%s\"\r\n", strings.NewReplacer("\r", "", "\n", "").Replace(ctype), mime.QEncoding.Encode("utf-8", name))
		b.WriteString("Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&b, "Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", mime.QEncoding.Encode("utf-8", name))
		enc := base64.StdEncoding.EncodeToString(a.Data)
		for len(enc) > 76 {
			b.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		b.WriteString(enc + "\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.String()
}

func newBoundary() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return "cc-" + hex.EncodeToString(buf)
}

// NoopSender discards mail (tests).
type NoopSender struct{}

// Send implements Sender.
func (NoopSender) Send(context.Context, Message) error { return nil }

var (
	_ Sender = (*SMTPSender)(nil)
	_ Sender = NoopSender{}
)
