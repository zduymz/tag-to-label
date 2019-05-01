package tag_to_label

import "time"

type Config struct {
	Master         string
	RequestTimeout time.Duration
	AWSAssumeRole  string
	AWSRegion      string
	AWSVPCId       string
	APIRetries     int

	// Just use for testing purpse
	AWSCredsFile   string
	KubeConfig     string
}
