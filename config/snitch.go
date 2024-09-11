package config

import "os"

type SnitchConfiguration struct {
	Url string
}

func NewSnitchConfiguration() *SnitchConfiguration {
	url := os.Getenv("SNITCH_URL")
	if url == "" {
		panic("Snitch URL not found")
	}

	return &SnitchConfiguration{
		Url: url,
	}
}
