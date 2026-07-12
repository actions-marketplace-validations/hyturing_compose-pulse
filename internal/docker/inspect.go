package docker

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/container"
)

const inspectCacheTTL = 2 * time.Second

type inspectCacheEntry struct {
	info *InspectInfo
	at   time.Time
}

// Inspect returns distilled inspect data, cached for 2s per container.
// Polling must not inspect every container — callers request on demand.
func (c *Client) Inspect(ctx context.Context, containerID string) (*InspectInfo, error) {
	c.inspectMu.Lock()
	if c.inspectCache == nil {
		c.inspectCache = make(map[string]inspectCacheEntry)
	}
	if e, ok := c.inspectCache[containerID]; ok && time.Since(e.at) < inspectCacheTTL {
		info := e.info
		c.inspectMu.Unlock()
		return info, nil
	}
	c.inspectMu.Unlock()

	raw, err := c.api.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}
	info := distillInspect(raw)

	c.inspectMu.Lock()
	c.inspectCache[containerID] = inspectCacheEntry{info: info, at: time.Now()}
	c.inspectMu.Unlock()
	return info, nil
}

func distillInspect(raw container.InspectResponse) *InspectInfo {
	info := &InspectInfo{ID: raw.ID}
	if raw.RestartCount != 0 {
		info.RestartCount = raw.RestartCount
	}
	if raw.HostConfig != nil {
		info.RestartPolicy = string(raw.HostConfig.RestartPolicy.Name)
	}
	if raw.Config != nil {
		info.Env = append([]string(nil), raw.Config.Env...)
		if raw.Config.Healthcheck != nil {
			hc := raw.Config.Healthcheck
			info.Healthcheck = &HealthcheckSpec{
				Test:        append([]string(nil), hc.Test...),
				Interval:    hc.Interval,
				Timeout:     hc.Timeout,
				StartPeriod: hc.StartPeriod,
				Retries:     hc.Retries,
			}
		}
	}
	if raw.State != nil {
		info.OOMKilled = raw.State.OOMKilled
		info.Error = raw.State.Error
		info.StartedAt = parseDockerTime(raw.State.StartedAt)
		info.FinishedAt = parseDockerTime(raw.State.FinishedAt)
		if raw.State.Health != nil {
			h := &HealthInfo{
				Status:        raw.State.Health.Status,
				FailingStreak: raw.State.Health.FailingStreak,
			}
			for _, r := range raw.State.Health.Log {
				if r == nil {
					continue
				}
				h.Log = append(h.Log, ProbeResult{
					Start:    r.Start,
					End:      r.End,
					ExitCode: r.ExitCode,
					Output:   r.Output,
				})
			}
			info.Health = h
		}
	}
	return info
}

// parseDockerTime parses RFC3339Nano inspect timestamps. Docker uses
// "0001-01-01T00:00:00Z" for "never" — treat year <= 1 as zero.
func parseDockerTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}
		}
	}
	if t.Year() <= 1 {
		return time.Time{}
	}
	return t
}
