package mail

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
)

// fakeSMTP answers just enough SMTP for one message and returns the DATA payload.
func fakeSMTP(ln net.Listener) <-chan string {
	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		write("220 fake ESMTP")
		var data strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					got <- data.String()
					write("250 queued")
					continue
				}
				data.WriteString(line)
				continue
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"):
				write("250-fake\r\n250 AUTH PLAIN")
			case strings.HasPrefix(cmd, "AUTH"):
				write("235 ok")
			case cmd == "DATA":
				inData = true
				write("354 go")
			case cmd == "QUIT":
				write("221 bye")
				return
			default:
				write("250 ok")
			}
		}
	}()
	return got
}

func addr(ln net.Listener) (string, int) {
	host, p, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(p)
	return host, port
}

var testMsg = Message{To: []string{"user@x.test"}, Subject: "Şifre sıfırlama", Body: "Merhaba\nkod: 123"}

func TestSendImplicitTLSWithAuth(t *testing.T) {
	srv := httptest.NewTLSServer(nil) // borrows a self-signed 127.0.0.1 certificate
	defer srv.Close()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", srv.TLS)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	got := fakeSMTP(ln)
	host, port := addr(ln)

	s := NewSMTPSender(config.SMTPConfig{Host: host, Port: port, Username: "u", Password: "p", From: "noreply@x.test", FromName: "ComCode", TLS: TLSImplicit})
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	s.rootCAs = roots

	if err := s.Send(context.Background(), testMsg); err != nil {
		t.Fatalf("send: %v", err)
	}
	payload := <-got
	if !strings.Contains(payload, "Subject: =?utf-8?q?") {
		t.Errorf("subject not Q-encoded:\n%s", payload)
	}
	if !strings.Contains(payload, "Merhaba\r\nkod: 123") {
		t.Errorf("body line endings not normalised:\n%s", payload)
	}
}

func TestAutoUsesImplicitTLSOn465(t *testing.T) {
	s := NewSMTPSender(config.SMTPConfig{Port: 465})
	if !s.implicitTLS() {
		t.Fatal("port 465 in auto mode should use implicit TLS")
	}
	if NewSMTPSender(config.SMTPConfig{Port: 587}).implicitTLS() {
		t.Fatal("port 587 in auto mode should not use implicit TLS")
	}
}

func TestSendPlainWithoutAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	got := fakeSMTP(ln)
	host, port := addr(ln)
	s := NewSMTPSender(config.SMTPConfig{Host: host, Port: port, From: "noreply@x.test", TLS: TLSNone})
	if err := s.Send(context.Background(), testMsg); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-got
}

func TestSendDoesNotHangOnSilentServer(t *testing.T) {
	// A plain client on an SMTPS port sees no greeting: the send must time out, not block forever.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			time.Sleep(3 * time.Second)
			_ = conn.Close()
		}
	}()
	host, port := addr(ln)
	s := NewSMTPSender(config.SMTPConfig{Host: host, Port: port, TLS: TLSNone})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := s.Send(ctx, testMsg); err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("send blocked for %s", elapsed)
	}
}

func TestPayloadWithAttachment(t *testing.T) {
	s := NewSMTPSender(config.SMTPConfig{From: "noreply@x.test"})
	vcf := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Ahmet Yılmaz\r\nEND:VCARD\r\n")
	p := s.payload("noreply@x.test", Message{
		To: []string{"a@x.test"}, Subject: "Kopya", Body: "Merhaba\nek var",
		Attachments: []Attachment{{Filename: "Ahmet_Yılmaz.vcf", ContentType: "text/vcard; charset=utf-8", Data: vcf}},
	})
	head, rest, ok := strings.Cut(p, "Content-Type: multipart/mixed; boundary=\"")
	if !ok || !strings.Contains(head, "MIME-Version: 1.0") {
		t.Fatalf("not multipart:\n%s", p)
	}
	boundary, _, _ := strings.Cut(rest, "\"")
	if !strings.Contains(p, "Merhaba\r\nek var\r\n--"+boundary+"\r\n") || !strings.HasSuffix(p, "--"+boundary+"--\r\n") {
		t.Fatalf("parts not delimited:\n%s", p)
	}
	if !strings.Contains(p, "Content-Disposition: attachment; filename=\"=?utf-8?q?") ||
		!strings.Contains(p, base64.StdEncoding.EncodeToString(vcf)[:60]) {
		t.Fatalf("attachment missing:\n%s", p)
	}
	if plain := s.payload("noreply@x.test", testMsg); !strings.Contains(plain, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("plain message changed:\n%s", plain)
	}
}
