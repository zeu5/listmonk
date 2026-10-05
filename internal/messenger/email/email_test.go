package email

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/knadh/listmonk/models"
	"github.com/knadh/smtppool/v2"
)

func TestSendUsesConfiguredWaitTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

		time.Sleep(3 * time.Second)
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		writeSMTPLine(t, rw, "220 localhost ESMTP")
		for {
			line, err := rw.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "), strings.HasPrefix(line, "HELO "):
				writeSMTPLine(t, rw, "250-localhost")
				writeSMTPLine(t, rw, "250 OK")
			case strings.HasPrefix(line, "MAIL FROM:"):
				writeSMTPLine(t, rw, "250 OK")
			case strings.HasPrefix(line, "RCPT TO:"):
				writeSMTPLine(t, rw, "250 OK")
			case strings.HasPrefix(line, "DATA"):
				writeSMTPLine(t, rw, "354 End data with <CR><LF>.<CR><LF>")
			case strings.TrimSpace(line) == ".":
				writeSMTPLine(t, rw, "250 OK")
			case strings.HasPrefix(line, "RSET"):
				writeSMTPLine(t, rw, "250 OK")
			case strings.HasPrefix(line, "QUIT"):
				writeSMTPLine(t, rw, "221 Bye")
				return
			}
		}
	}()

	host, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatal(err)
	}

	msgr, err := New("", Server{
		AuthProtocol: "none",
		TLSType:      "none",
		Opt: smtppool.Opt{
			Host:              host,
			Port:              port,
			MaxConns:          1,
			MaxMessageRetries: 1,
			PoolWaitTimeout:   5 * time.Second,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := msgr.Push(models.Message{
		From:    "from@example.com",
		To:      []string{"to@example.com"},
		Subject: "test",
		Body:    []byte("ok"),
	}); err != nil {
		t.Fatalf("Push returned after %s: %v", time.Since(start), err)
	}
	if err := msgr.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP server did not finish")
	}
}

func writeSMTPLine(t *testing.T, rw *bufio.ReadWriter, line string) {
	t.Helper()
	if _, err := rw.WriteString(line + "\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Flush(); err != nil {
		t.Fatal(err)
	}
}
