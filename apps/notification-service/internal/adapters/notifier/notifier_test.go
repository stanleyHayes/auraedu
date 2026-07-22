package notifier

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
)

func TestProductionRejectsMockProvider(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("NOTIFICATION_PROVIDER", "mock")
	if _, err := RegistryFromEnv(); err == nil {
		t.Fatal("production must reject mock notifications")
	}
}

func TestSMTPConfigurationRequired(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("NOTIFICATION_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_FROM_EMAIL", "")
	if _, err := RegistryFromEnv(); err == nil {
		t.Fatal("SMTP configuration must be required")
	}
}

func TestProductionRejectsInsecureSMTP(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("NOTIFICATION_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_FROM_EMAIL", "notifications@example.test")
	t.Setenv("SMTP_ALLOW_INSECURE", "true")
	if _, err := RegistryFromEnv(); err == nil || !strings.Contains(err.Error(), "forbidden in production") {
		t.Fatalf("production must require TLS, got %v", err)
	}
}

func TestLocalDefaultsToMocks(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("NOTIFICATION_PROVIDER", "")
	t.Setenv("SMTP_HOST", "")
	registry, err := RegistryFromEnv()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if _, ok := registry["email"].(*MockNotifier); !ok {
		t.Fatalf("expected local mock email adapter, got %T", registry["email"])
	}
}

func TestSMTPNotifierDeliversStableEnvelopeToProvider(t *testing.T) {
	host, port, received := startSMTPTestServer(t)
	n := NewSMTPNotifier(SMTPConfig{
		Host: host, Port: port, From: "notifications@auraedu.test", FromName: "AuraEDU",
		AllowInsecure: true,
	})
	message := domain.Message{
		ID: "0198f0db-7d3d-7000-8000-000000000001", TenantID: "upshs", RecipientID: "user-1",
		Channel: "email", Subject: "Welcome\r\nBcc: attacker@example.test", Body: "Your school is ready.",
		Metadata: map[string]any{"delivery_address": "founder@example.test"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.Send(ctx, message); err != nil {
		t.Fatalf("send: %v", err)
	}
	payload := <-received
	for _, want := range []string{
		"MAIL FROM:<notifications@auraedu.test>", "RCPT TO:<founder@example.test>",
		"Message-ID: <0198f0db-7d3d-7000-8000-000000000001@auraedu>",
		"Subject: Welcome  Bcc: attacker@example.test", "Your school is ready.",
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("provider payload missing %q:\n%s", want, payload)
		}
	}
	if strings.Contains(payload, "\r\nBcc: attacker@example.test\r\n") {
		t.Fatal("subject header injection reached the provider")
	}
}

func TestSMTPNotifierRequiresSTARTTLSByDefault(t *testing.T) {
	host, port, _ := startSMTPTestServer(t)
	n := NewSMTPNotifier(SMTPConfig{Host: host, Port: port, From: "notifications@auraedu.test"})
	err := n.Send(context.Background(), domain.Message{
		ID: "message", RecipientID: "founder@example.test", Subject: "Welcome", Body: "Body",
	})
	if err == nil || !strings.Contains(err.Error(), "does not offer STARTTLS") {
		t.Fatalf("plaintext provider must fail closed, got %v", err)
	}
}

func TestSMTPNotifierNegotiatesTrustedSTARTTLS(t *testing.T) {
	host, port, roots, received := startSTARTTLSTestServer(t)
	n := NewSMTPNotifier(SMTPConfig{
		Host: host, Port: port, From: "notifications@auraedu.test", FromName: "AuraEDU",
	})
	n.rootCAs = roots
	if err := n.Send(context.Background(), domain.Message{
		ID: "0198f0db-7d3d-7000-8000-000000000002", RecipientID: "founder@example.test",
		Subject: "Secure welcome", Body: "Delivered over TLS.",
	}); err != nil {
		t.Fatalf("send over STARTTLS: %v", err)
	}
	delivery := <-received
	if delivery.tlsVersion < tls.VersionTLS12 {
		t.Fatalf("TLS version = %x, want TLS 1.2+", delivery.tlsVersion)
	}
	if !strings.Contains(delivery.payload, "Delivered over TLS.") {
		t.Fatalf("secure provider did not receive payload: %s", delivery.payload)
	}
}

func TestSMTPNotifierHonoursContextDeadlineDuringProtocol(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	closeListenerOnCleanup(t, listener)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer closeConnection(conn)
		if _, writeErr := conn.Write([]byte("220 smtp.test ESMTP ready\r\n")); writeErr != nil {
			return
		}
		if _, readErr := bufio.NewReader(conn).ReadString('\n'); readErr != nil {
			return
		}
		<-time.After(time.Second)
	}()
	host, rawPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split address: %v", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	n := NewSMTPNotifier(SMTPConfig{Host: host, Port: port, From: "notifications@auraedu.test"})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = n.Send(ctx, domain.Message{ID: "message", RecipientID: "founder@example.test", Subject: "Welcome", Body: "Body"})
	if err == nil {
		t.Fatal("stalled SMTP provider must fail on the request deadline")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("SMTP cancellation took %s", elapsed)
	}
}

type smtpTLSDelivery struct {
	payload    string
	tlsVersion uint16
}

func startSTARTTLSTestServer(t *testing.T) (string, int, *x509.CertPool, <-chan smtpTLSDelivery) {
	t.Helper()
	certificate, roots := testSMTPServerCertificate(t)
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	closeListenerOnCleanup(t, listener)
	host, rawPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split address: %v", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	received := make(chan smtpTLSDelivery, 1)
	go func() {
		plain, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer closeConnection(plain)
		reader := bufio.NewReader(plain)
		writer := bufio.NewWriter(plain)
		if !writeAndFlush(writer, "220 smtp.test ESMTP ready\r\n") {
			return
		}
		secure := false
		var tlsVersion uint16
		var transcript strings.Builder
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			command := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(command, "EHLO ") && !secure:
				if !writeString(writer, "250-smtp.test\r\n250 STARTTLS\r\n") {
					return
				}
			case strings.HasPrefix(command, "EHLO ") && secure:
				if !writeString(writer, "250 smtp.test\r\n") {
					return
				}
			case command == "STARTTLS":
				if !writeAndFlush(writer, "220 ready for TLS\r\n") {
					return
				}
				tlsConn := tls.Server(plain, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
				if handshakeErr := tlsConn.HandshakeContext(context.Background()); handshakeErr != nil {
					return
				}
				secure = true
				tlsVersion = tlsConn.ConnectionState().Version
				reader = bufio.NewReader(tlsConn)
				writer = bufio.NewWriter(tlsConn)
			case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
				transcript.WriteString(command + "\n")
				if !writeString(writer, "250 accepted\r\n") {
					return
				}
			case command == "DATA":
				if !writeAndFlush(writer, "354 end with <CRLF>.<CRLF>\r\n") {
					return
				}
				for {
					dataLine, dataErr := reader.ReadString('\n')
					if dataErr != nil {
						return
					}
					if dataLine == ".\r\n" {
						break
					}
					transcript.WriteString(dataLine)
				}
				received <- smtpTLSDelivery{payload: transcript.String(), tlsVersion: tlsVersion}
				if !writeString(writer, "250 queued\r\n") {
					return
				}
			case command == "QUIT":
				if !writeAndFlush(writer, "221 bye\r\n") {
					return
				}
				return
			default:
				if !writeString(writer, "500 unsupported\r\n") {
					return
				}
			}
			if !flushWriter(writer) {
				return
			}
		}
	}()
	return host, port, roots, received
}

func testSMTPServerCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certificate := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	roots := x509.NewCertPool()
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	roots.AddCert(parsed)
	return certificate, roots
}

func startSMTPTestServer(t *testing.T) (string, int, <-chan string) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	closeListenerOnCleanup(t, listener)
	host, rawPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split address: %v", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	received := make(chan string, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer closeConnection(conn)
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		if !writeAndFlush(writer, "220 smtp.test ESMTP ready\r\n") {
			return
		}
		var transcript strings.Builder
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			command := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(command, "EHLO "), strings.HasPrefix(command, "HELO "):
				if !writeString(writer, "250 smtp.test\r\n") {
					return
				}
			case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
				transcript.WriteString(command + "\n")
				if !writeString(writer, "250 accepted\r\n") {
					return
				}
			case command == "DATA":
				if !writeAndFlush(writer, "354 end with <CRLF>.<CRLF>\r\n") {
					return
				}
				for {
					dataLine, dataErr := reader.ReadString('\n')
					if dataErr != nil {
						return
					}
					if dataLine == ".\r\n" {
						break
					}
					transcript.WriteString(dataLine)
				}
				received <- transcript.String()
				if !writeString(writer, "250 queued\r\n") {
					return
				}
			case command == "QUIT":
				if !writeAndFlush(writer, "221 bye\r\n") {
					return
				}
				return
			default:
				if !writeString(writer, fmt.Sprintf("500 unsupported %s\r\n", command)) {
					return
				}
			}
			if !flushWriter(writer) {
				return
			}
		}
	}()
	return host, port, received
}

func writeString(writer *bufio.Writer, value string) bool {
	_, err := writer.WriteString(value)
	return err == nil
}

func flushWriter(writer *bufio.Writer) bool {
	return writer.Flush() == nil
}

func writeAndFlush(writer *bufio.Writer, value string) bool {
	return writeString(writer, value) && flushWriter(writer)
}

func closeListenerOnCleanup(t *testing.T, listener net.Listener) {
	t.Helper()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			t.Errorf("close SMTP listener: %v", err)
		}
	})
}

func closeConnection(connection net.Conn) {
	if err := connection.Close(); err != nil {
		return
	}
}
