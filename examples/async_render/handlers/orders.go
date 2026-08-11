package handlers

// The work the page awaits. It takes a context rather than a request, so it is
// not derived for the second build and both compile it — which is why it sits
// here rather than beside the handlers a build tag excludes.

import (
	"context"
	"errors"
	"time"
)

func loadOrders(ctx context.Context, fail bool) ([]Order, error) {
	if err := sleep(ctx, 900*time.Millisecond); err != nil {
		return nil, err
	}
	if fail {
		return nil, errors.New("order service returned 503")
	}
	return []Order{
		{Id: "A-1043", Total: "¥12,800"},
		{Id: "A-1088", Total: "¥3,200"},
		{Id: "A-1120", Total: "¥45,000"},
	}, nil
}

// recommend is the slower of the two dependencies, and the one with a recover
// clause behind it. The error it returns never reaches the page: a recover
// subtree sees a safe pw.AsyncError, and this text goes to the log instead.
func recommend(ctx context.Context, fail bool) (string, error) {
	if err := sleep(ctx, 1500*time.Millisecond); err != nil {
		return "", err
	}
	if fail {
		return "", errors.New("recommendation service returned 503")
	}
	return "Because you bought A-1043, you may like the analytical engine starter kit.", nil
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
