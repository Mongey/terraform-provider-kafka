package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// More "Updating" polls than the SDK's NotFoundChecks default (20) must not fail the wait
// while Timeout has not been reached.
func TestRFUpdateRefreshFunc_ManyUpdatingPollsDoNotFail(t *testing.T) {
	polls := 0
	refresh := rfUpdateRefreshFunc("topic", func() (bool, error) {
		polls++
		return polls <= 30, nil
	})

	stateConf := &retry.StateChangeConf{
		Pending:      []string{"Updating"},
		Target:       []string{"Ready"},
		Refresh:      refresh,
		Timeout:      10 * time.Second,
		PollInterval: time.Millisecond,
	}

	if _, err := stateConf.WaitForStateContext(context.Background()); err != nil {
		t.Fatalf("expected wait to succeed after %d polls, got: %s", polls, err)
	}
}

func TestRFUpdateRefreshFunc_Error(t *testing.T) {
	want := errors.New("boom")
	_, state, err := rfUpdateRefreshFunc("topic", func() (bool, error) { return false, want })()

	if state != "Error" || !errors.Is(err, want) {
		t.Fatalf("expected Error state with %v, got %q, %v", want, state, err)
	}
}
