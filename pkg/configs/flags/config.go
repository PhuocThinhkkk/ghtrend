package flags

import (
	"fmt"
)

type Frequency string

const (
	Weekly  Frequency = "weekly"
	Monthly Frequency = "monthly"
	Daily   Frequency = "daily"
)

func IsValidFrequency(f Frequency) bool {
	switch f {
	case Weekly, Monthly, Daily:
		return true
	default:
		return false
	}
}

// NextFrequency cycles between Daily -> Weekly -> Monthly -> Daily.
func NextFrequency(f Frequency) Frequency {
	switch f {
	case Daily:
		return Weekly
	case Weekly:
		return Monthly
	case Monthly:
		return Daily
	default:
		return Daily
	}
}

// DisplayTitle returns the capitalized display string for a Frequency.
func (f Frequency) DisplayTitle() string {
	switch f {
	case Weekly:
		return "Weekly"
	case Monthly:
		return "Monthly"
	default:
		return "Daily"
	}
}

type CmdConfig struct {
	IsCache    bool
	Limit      int
	Since      Frequency
	Language   string
	Theme      string
	User       string
	LaunchHome bool
	DirectRepo string
}

func NewConfig(cache bool, limit int, since Frequency, language string) (*CmdConfig, error) {
	config := &CmdConfig{
		IsCache:  cache,
		Limit:    limit,
		Since:    since,
		Language: language,
		Theme:    "",
		User:     "",
	}
	if !IsValidFrequency(since) {
		return config, fmt.Errorf("invalid since value: since have to be one of these 'daily', 'weekly' or 'monthly'")
	}
	if limit <= 0 {
		return config, fmt.Errorf("invalid limit value: limit have to be a positive number")
	}
	return config, nil
}
