package config

import "os"

type SnitchConfiguration struct {
	Url string
}

func NewSnitchConfiguration() *SnitchConfiguration {
	url := os.Getenv("SNITCH_URL")

	return &SnitchConfiguration{
		Url: url,
	}
}
