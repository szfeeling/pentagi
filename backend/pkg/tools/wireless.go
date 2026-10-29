package tools

import (
	"context"
	"fmt"
	"time"

	"pentagi/pkg/docker"
	"pentagi/pkg/wireless"

	"github.com/sirupsen/logrus"
)

func (fte *flowToolsExecutor) wirelessGuard(ctx context.Context) (context.Context, func(), error) {
	if fte.functions == nil || fte.functions.Wireless == nil {
		return ctx, func() {}, nil
	}
	if fte.cfg == nil || fte.cfg.Wireless == nil {
		return ctx, func() {}, wireless.ErrEnvironmentChanged
	}
	binding := *fte.functions.Wireless
	network, err := fte.cfg.Wireless.Check(ctx, fte.userID, binding)
	if err != nil {
		return ctx, func() {}, fmt.Errorf("%w: %v", wireless.ErrEnvironmentChanged, err)
	}
	if fte.primaryLID != "" {
		if err := docker.CheckWirelessContainer(ctx, fte.docker, fte.primaryLID, network); err != nil {
			return ctx, func() {}, fmt.Errorf("%w: %v", wireless.ErrEnvironmentChanged, err)
		}
	}
	guarded, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-guarded.Done():
				return
			case <-ticker.C:
				probe, done := context.WithTimeout(guarded, 5*time.Second)
				_, err := fte.cfg.Wireless.Check(probe, fte.userID, binding)
				done()
				if err == nil {
					continue
				}
				if guarded.Err() != nil {
					return
				}
				cancel()
				cleanup, finish := context.WithTimeout(context.Background(), 10*time.Second)
				defer finish()
				if fte.primaryLID != "" {
					_ = fte.docker.StopContainer(cleanup, fte.primaryLID, fte.primaryID)
				}
				return
			}
		}
	}()
	return guarded, cancel, nil
}

func (fte *flowToolsExecutor) startWirelessWatch() {
	if fte.functions == nil || fte.functions.Wireless == nil || fte.cfg.Wireless == nil {
		return
	}
	fte.wirelessWatchOnce.Do(func() {
		binding := *fte.functions.Wireless
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-fte.wirelessStop:
					return
				case <-ticker.C:
					probe, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					_, err := fte.cfg.Wireless.Check(probe, fte.userID, binding)
					cancel()
					if err == nil {
						continue
					}
					cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
					defer done()
					if err := fte.docker.StopContainer(cleanup, fte.primaryLID, fte.primaryID); err != nil {
						logrus.WithError(err).WithField("flow_id", fte.flowID).Error("failed to stop invalidated wireless runtime")
					}
					return
				}
			}
		}()
	})
}
