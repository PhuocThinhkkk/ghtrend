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


type CmdConfig struct {
	IsCache  bool
	Limit int
	Since Frequency
	Language string
}

func NewConfig(cache bool, limit int, since Frequency, language string) (*CmdConfig, error){
	config := &CmdConfig{
		IsCache: cache,
		Limit: limit,
		Since: since,
		Language: language,
	}
	if !IsValidFrequency(since) {
		return config, fmt.Errorf("invalid since value: since have to be one of these 'daily', 'weekly' or 'monthly'")
	}
	if limit <= 0 {
		return config, fmt.Errorf("invalid limit value: limit have to be a positive number")
	}
	return config, nil
}
