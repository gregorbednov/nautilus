package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	nio "github.com/joyautomation/nautilus/io"
	"github.com/joyautomation/nautilus/omron/monitor"
)

func watchOmron(ctx context.Context, dial monitor.Dial, source string, poll, stale time.Duration) error {
	if source == "" || poll <= 0 {
		return errors.New("watch requires -tags FILE|- and a positive polling period")
	}
	input := os.Stdin
	if source != "-" {
		file, err := os.Open(source)
		if err != nil {
			return err
		}
		defer file.Close()
		input = file
	}
	const bindingLimit = 8 << 20
	data, err := io.ReadAll(io.LimitReader(input, bindingLimit+1))
	if err != nil {
		return err
	}
	if len(data) > bindingLimit {
		return errors.New("watch binding data exceeds size limit")
	}
	var bindings []monitor.Binding
	if err := json.Unmarshal(data, &bindings); err != nil {
		return err
	}
	driver, err := monitor.New(ctx, dial, bindings, stale)
	if err != nil {
		return err
	}
	defer driver.Close()
	return streamOmronWatch(ctx, driver, os.Stdout, poll)
}

type omronWatchSample struct {
	CPU     monitor.Health    `json:"cpu"`
	Values  nio.Values        `json:"values"`
	Quality map[string]string `json:"quality"`
}

func streamOmronWatch(ctx context.Context, driver *monitor.Driver, output io.Writer, poll time.Duration) error {
	enc := json.NewEncoder(output)
	for ctx.Err() == nil {
		values, err := driver.ReadInputs()
		if err != nil {
			return err
		}
		sample := omronWatchSample{driver.Health(), values, omronWatchQuality(values, driver.Quality())}
		if err := enc.Encode(sample); err != nil {
			return err
		}
		timer := time.NewTimer(poll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
	return nil
}

func omronWatchQuality(values nio.Values, overrides map[string]nio.Quality) map[string]string {
	quality := make(map[string]string, len(values))
	for key := range values {
		quality[key] = nio.Good.String()
	}
	for key, state := range overrides {
		quality[key] = state.String()
	}
	return quality
}
