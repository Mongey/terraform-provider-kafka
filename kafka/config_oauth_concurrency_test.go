package kafka

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type concurrentOAuthConfig struct {
	calls atomic.Int32
}

func (c *concurrentOAuthConfig) Token(context.Context) (*oauth2.Token, error) {
	c.calls.Add(1)
	time.Sleep(20 * time.Millisecond)
	return &oauth2.Token{AccessToken: "shared", Expiry: time.Now().Add(time.Hour)}, nil
}

func TestOauthbearerTokenProviderConcurrentRefresh(t *testing.T) {
	config := &concurrentOAuthConfig{}
	provider := newOauthbearerTokenProvider(config)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			token, err := provider.Token()
			if err != nil || token.Token != "shared" {
				t.Errorf("Token() = %v, %v; want shared token", token, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := config.calls.Load(); got != 1 {
		t.Errorf("refresh calls = %d; want 1", got)
	}
}
