package kafka

import (
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

func Test_LazyClientWithNoConfig(t *testing.T) {
	c := &LazyClient{}
	_, err := c.ListACLs()

	if err == nil {
		t.Fatalf("exepted err, got %v", err)
	}
}

func Test_LazyClientErrors(t *testing.T) {
	c := &LazyClient{}
	_, err := c.ListACLs()
	if err == nil {
		t.Fatalf("exepted err, got %v", err)
	}
	_, err = c.ListACLs()
	if err == nil {
		t.Fatalf("exepted err, got %v", err)
	}
}

func Test_LazyClientWithConfigErrors(t *testing.T) {
	config := &Config{
		BootstrapServers: &[]string{"localhost:9000"},
		Timeout:          10,
	}
	c := &LazyClient{
		Config: config,
	}
	err := c.init()

	if !strings.Contains(err.Error(), sarama.ErrOutOfBrokers.Error()) {
		t.Fatalf("expected err, got %v", err)
	}
}

// Test_LazyClientConcurrentInit tests that concurrent calls to init() do not
// cause a data race. This test should be run with the -race flag to detect
// any race conditions in the lazy initialization.
func Test_LazyClientConcurrentInit(t *testing.T) {
	config := &Config{
		BootstrapServers: &[]string{"localhost:9000"},
		Timeout:          10,
	}
	c := &LazyClient{
		Config: config,
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Spawn multiple goroutines calling init() simultaneously
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			// Call init() - this should not cause a data race
			_ = c.init()
		}()
	}

	wg.Wait()

	// Verify that the error is set correctly (should be ErrOutOfBrokers since no broker is available)
	if c.initErr == nil || !strings.Contains(c.initErr.Error(), sarama.ErrOutOfBrokers.Error()) {
		t.Fatalf("expected ErrOutOfBrokers, got %v", c.initErr)
	}
}

// Test_checkTLSConfigClosesConnection verifies that checkTLSConfig properly closes
// the TLS connection after performing the handshake check.
// This test creates a mock TLS server and tracks whether connections are closed.
func Test_checkTLSConfigClosesConnection(t *testing.T) {
	// Track connection state
	var connectionClosed atomic.Bool
	connectionClosed.Store(false)

	// Create a TLS certificate for the test server
	cert, err := tls.X509KeyPair([]byte(testServerCert), []byte(testServerKey))
	if err != nil {
		t.Fatalf("failed to load test certificate: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Start a test TLS server
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("failed to create TLS listener: %v", err)
	}
	defer listener.Close()

	// Get the address the server is listening on
	addr := listener.Addr().String()

	// Handle connections in a goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Listener was closed
			}
			// Wrap to detect when connection is closed
			go func(c net.Conn) {
				// Perform TLS handshake
				tlsConn := c.(*tls.Conn)
				if err := tlsConn.Handshake(); err != nil {
					c.Close()
					return
				}
				// Read until EOF (connection closed by client)
				buf := make([]byte, 1)
				for {
					_, err := c.Read(buf)
					if err != nil {
						connectionClosed.Store(true)
						return
					}
				}
			}(conn)
		}
	}()

	// Create a LazyClient with TLS enabled pointing to our test server
	config := &Config{
		BootstrapServers: &[]string{addr},
		Timeout:          10,
		TLSEnabled:       true,
		SkipTLSVerify:    true, // Skip verification since we're using a self-signed cert
	}

	client := &LazyClient{
		Config: config,
	}

	// Call checkTLSConfig - this should connect, handshake, and then close the connection
	err = client.checkTLSConfig()
	if err != nil {
		t.Fatalf("checkTLSConfig failed: %v", err)
	}

	// Give the server a moment to detect the connection close
	// The connection should be closed almost immediately if defer conn.Close() is used
	for i := 0; i < 100; i++ {
		if connectionClosed.Load() {
			break
		}
		// Small sleep to allow the close to propagate
		// In a proper implementation with defer conn.Close(), this should happen very quickly
		time.Sleep(10 * time.Millisecond)
	}

	// Verify the connection was closed
	if !connectionClosed.Load() {
		t.Error("TLS connection was not closed after checkTLSConfig - this is a resource leak!")
	}
}

// Test certificates for the mock TLS server
// These are self-signed certificates for testing purposes only
const testServerCert = `-----BEGIN CERTIFICATE-----
MIIDCTCCAfGgAwIBAgIUeDLGPxZh5iKtBEGbFNv5KuVIqSwwDQYJKoZIhvcNAQEL
BQAwFDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTI1MTIyODE1MTEzNVoXDTI2MTIy
ODE1MTEzNVowFDESMBAGA1UEAwwJbG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEAwquTlLBltGQZgYy/k/dVzXp6tSs615rHPEzyfnhk7+Hj
UDvkf1rJfT347aaOaB7r56iMH4EY/VJJ5uRa1gk5y9CRoQwhCQvfYhG7i8X+dgUn
KEu1KSzJpKUCqLehZsb0lJFYnGMsvqvx5gQaMOMNIjsapa96Z7rK9zV1HhWSXOUs
0CUYXeSZVkaCVS5/G7SHFH+mJvuo3ykq8RF56xTMG7P8Sdqv5oem7k848rL9BXid
WG9AU1RTwt5DXl68Yyj+AEpW1ItipB8xgvGdAJ3FQUOofCMm468KXxZf5sqXjAZG
vp3zJaMsOwnsaEmJ0Pv/isyN5V7+e7KO4w4rO70xLwIDAQABo1MwUTAdBgNVHQ4E
FgQU3aM51ZxFmqUaUIjY8eh+UORcR4YwHwYDVR0jBBgwFoAU3aM51ZxFmqUaUIjY
8eh+UORcR4YwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAGLza
ei0wGZyBDiUVHK/F4o/Qgm2EJiI5jYWb6a5LIDzT07LaM036D71KF+AVyHFo369P
u8nAWqDt4Mp0PWqAxc9GPW+V8OkBoJd5EYtIUgqygi+uonIBsdF8DdZ9cKuE4K+r
kmopFxNGDlotTtF5D/QN2Bdd5m+43y7/tTCtparFkSZ9uwd4350/bZEz3Eq2F4qI
vUUoxSdeu0J9SR9U8DTlNfveTrOY5ZQUo/iABGyZEhuqjDN5RUcaLZxjxs39xPli
9CQDBP2/ooEX6KZkVCFJqV9GtdM6tMgrUsRrVQl9z7X00p/D7WH3iRLcLJppNwMF
YbZHVLyoMg3Xbete+w==
-----END CERTIFICATE-----`

const testServerKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDCq5OUsGW0ZBmB
jL+T91XNenq1KzrXmsc8TPJ+eGTv4eNQO+R/Wsl9Pfjtpo5oHuvnqIwfgRj9Uknm
5FrWCTnL0JGhDCEJC99iEbuLxf52BScoS7UpLMmkpQKot6FmxvSUkVicYyy+q/Hm
BBow4w0iOxqlr3pnusr3NXUeFZJc5SzQJRhd5JlWRoJVLn8btIcUf6Ym+6jfKSrx
EXnrFMwbs/xJ2q/mh6buTzjysv0FeJ1Yb0BTVFPC3kNeXrxjKP4ASlbUi2KkHzGC
8Z0AncVBQ6h8IybjrwpfFl/mypeMBka+nfMloyw7CexoSYnQ+/+KzI3lXv57so7j
Dis7vTEvAgMBAAECggEAHTIoZyNxjXV50dEvJlzw9GlLIALEx3NCMEwGDmu2D7gc
JHtnEKaoE22I+POC5iDFFrBTm6H8Anol9UgIS5OEpIm6XaH5DmdGcGnia9sdB8xM
DCIWoH9EGrpYxL8NqOFr6yBFXucM3efh1rKEzxIudRTSMUk5HXeJWzwcPY/UrLO7
GDEmZJGPkCiym4ckMlhjdN0rDscOozg5mTR/vxU21Y7hOSJDqjpAs8biLGJVTAco
W37I2qMGg8brXpuOJw10zSoEu5s1tNiar8/fjpi7ZntLLVRDRLr8k26ZzVa9IrYS
GuPMGP/R5oHVbyhBgVqFPdyAzDaP0tDuNOn6JM14KQKBgQD1xudja9sImrfjPmPw
AdbNBHb/VGwNHHvlfu5DehEY9UPH724RQtooyke1qizwmdZ3fxFyJXlLuj1C/Hi7
UaWPx+TlkVHpBWgm57JdJIr895QlBu8W8KGNaFRviLL+gxs9Ptz3TnYMOzYXyYhE
frw6zABKoIp7fiiPCNDJyEmsYwKBgQDKxHmR5U1o/qSpcv+YNEl0pSdByz09/Wmp
hutSD3m9Vv6MVYqGZ4PZFyXODXcvVn+uBfp8OrmjI5nBzsUkIr1VTgTDwhEmEKEf
lpIDFWRHR47PNSV2GX0wU9bSPBAVmSBAqgctJlSPcGBfd+CWC8ux4OXfzxh9OXnT
HE/lvb4jxQKBgAWZW2obaej/RVMq97HfCNqw0FkuvitqS7RFuP3WiQ8tfzbN0I8a
G8g0G4Aa+V0d1BHy1h3olqPQAVdGUyXJTWFCJ4fHULtjQSUpwBl5HKV4qmpRhx7Z
qoSDLPFBhvpfWD6D8Rq9MdlDfA78q1sMHBOm1BbfI2h+zkO76q2+H1eLAoGBALJv
znAWs1Wnaa54tfbyZIYS5IYg3acUwAxg3+taFQ8LZHyItpvqsnuzxCAdd3ogC8JQ
Hot+fmjTZnbIiHJxY96TBtxihwbRcYlDzwCJrbKQhVtRcMMKUUHbNdvS4XCwTVK6
jhAsgBOumBDLhMdmX/4MZR7ct7dTgiLG8oTBwnblAoGBAJ0legPbiIfqkC08Ys1/
IzIdXmqer5ZdBKQDU82bHhM/Dyx0dZKKVA5CZD8Y3Jmz1G0/sOsMMjAwCiatavAz
RvJsx6SeQNRrXVPhQvc+RsTOiqr3JutsbBH81ZeGtBWlPs2DLfcsBW6UqiA8hRGr
knHTqlE3hYVMHqL+xJbKmPk7
-----END PRIVATE KEY-----`
