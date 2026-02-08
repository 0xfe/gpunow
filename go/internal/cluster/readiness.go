package cluster

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultReadinessTimeout = 15 * time.Minute
	readinessPort           = 34223
)

func (s *Service) WaitForReady(ctx context.Context, instanceName, host string, timeout time.Duration) error {
	if host == "" {
		return fmt.Errorf("instance %s has no external IP", instanceName)
	}
	if timeout <= 0 {
		timeout = defaultReadinessTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	state, err := probeReadinessHTTP(waitCtx, host)
	if err != nil {
		return fmt.Errorf("wait for readiness on %s: %w", instanceName, err)
	}
	if state != "ready" {
		return fmt.Errorf("instance %s reported readiness state %q", instanceName, state)
	}
	return nil
}

func probeReadinessHTTP(ctx context.Context, host string) (string, error) {
	probeURL := fmt.Sprintf("http://%s:%d/", host, readinessPort)
	_, err := url.Parse(probeURL)
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if reqErr != nil {
			return "", reqErr
		}
		resp, doErr := client.Do(req)
		if doErr == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32))
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK {
				state := strings.TrimSpace(strings.ToLower(string(body)))
				switch state {
				case "ready":
					return "ready", nil
				case "running":
					lastErr = fmt.Errorf("readiness still running")
				case "error":
					return "error", nil
				default:
					lastErr = fmt.Errorf("unexpected readiness response %q", state)
				}
			} else if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("unexpected readiness HTTP status %d", resp.StatusCode)
			}
		} else {
			lastErr = doErr
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return "", fmt.Errorf("%w: %v", ctx.Err(), lastErr)
			}
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}
