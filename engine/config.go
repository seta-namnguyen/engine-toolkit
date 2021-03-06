package main

import (
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// Config holds engine configuration settings.
type Config struct {
	// EndIfIdleDuration is the duration after the last message
	// at which point the engine will shut down.
	EndIfIdleDuration time.Duration
	// Stdout is the Engine's stdout. Subprocesses inherit this.
	Stdout io.Writer
	// Stderr is the Engine's stderr. Subprocesses inherit this.
	Stderr io.Writer
	// Subprocess holds configuration relating to the subprocess
	// that this engine supervises.
	Subprocess struct {
		// Arguments are the command line arguments (including the command as the
		// first argument) for the subprocess.
		// By default, these are taken from the arguments passed to this tool.
		Arguments []string
		// ReadyTimeout is the amount of time to wait before deciding that the subprocess
		// is not going to be ready.
		ReadyTimeout time.Duration
	}
	// Kafka holds Kafka configuration.
	Kafka struct {
		// Brokers is a list of Kafka brokers.
		Brokers []string
		// ConsumerGroup is the group name of the consumers.
		ConsumerGroup string
		// InputTopic is the topic on which chunks are received.
		InputTopic string
		// ChunkTopic is the output topic where chunk results are sent.
		ChunkTopic string
	}
	// Webhooks holds webhook addresses.
	Webhooks struct {
		// Ready holds configuration for the readiness webhook.
		Ready struct {
			// URL is the address of the Readiness Webhook.
			URL string
			// PollDuration is how often the URL will be polled to check
			// for readiness before processing begins.
			PollDuration time.Duration
			// MaximumPollDuration is the maximum of time to allow for the
			// engine to become ready before abandoning the operation altogether.
			MaximumPollDuration time.Duration
		}
		// Process holds configuration for the processing webhook.
		Process struct {
			// URL is the address of the Processing Webhook.
			URL string
		}
		// Backoff controls webhook backoff and retry policy.
		Backoff struct {
			// MaxRetries is the maximum number of retries that will be made before
			// giving up.
			MaxRetries int
			// InitialBackoffDuration is the time to wait before the first retry.
			InitialBackoffDuration time.Duration
			// MaxBackoffDuration is the maximum amount of time to wait before retrying.
			MaxBackoffDuration time.Duration
		}
	}
	// Tasks holds task configuration values.
	Tasks struct {
		// ProcessingUpdateInterval is how often the chunk processing update
		// is sent.
		ProcessingUpdateInterval time.Duration
	}
}

// NewConfig gets default configuration settings.
func NewConfig() Config {
	var c Config

	c.Tasks.ProcessingUpdateInterval = 90 * time.Second

	c.Subprocess.Arguments = os.Args[1:]
	c.Subprocess.ReadyTimeout = 1 * time.Minute
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.Webhooks.Ready.URL = os.Getenv("VERITONE_WEBHOOK_READY")
	c.Webhooks.Ready.PollDuration = 1 * time.Second
	c.Webhooks.Ready.MaximumPollDuration = 1 * time.Minute
	c.Webhooks.Process.URL = os.Getenv("VERITONE_WEBHOOK_PROCESS")
	c.Webhooks.Backoff.MaxRetries = 10
	c.Webhooks.Backoff.InitialBackoffDuration = 100 * time.Millisecond
	c.Webhooks.Backoff.MaxBackoffDuration = 1 * time.Second

	// veritone platform configuration
	if endSecs := os.Getenv("END_IF_IDLE_SECS"); endSecs != "" {
		var err error
		c.EndIfIdleDuration, err = time.ParseDuration(endSecs + "s")
		if err != nil {
			log.Printf("END_IF_IDLE_SECS %q: %v", endSecs, err)
		}
	}
	if c.EndIfIdleDuration == 0 {
		c.EndIfIdleDuration = 1 * time.Minute
	}

	// kafka configuration
	c.Kafka.Brokers = strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	c.Kafka.ConsumerGroup = os.Getenv("KAFKA_CONSUMER_GROUP")
	c.Kafka.InputTopic = os.Getenv("KAFKA_INPUT_TOPIC")
	c.Kafka.ChunkTopic = os.Getenv("KAFKA_CHUNK_TOPIC")

	return c
}
