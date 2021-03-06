package main

import (
	"io"
	"log"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/pkg/errors"
)

// Consumer consumes incoming messages.
type Consumer interface {
	io.Closer
	// Messages gets a channel on which sarama.ConsumerMessage
	// objets are sent. Closed when the Consumer is closed.
	Messages() <-chan *sarama.ConsumerMessage
	// MarkOffset indicates that this message has been processed.
	MarkOffset(msg *sarama.ConsumerMessage, metadata string)
}

// Producer produces outgoing messages.
type Producer interface {
	io.Closer
	// SendMessage sends a sarama.ProducerMessage.
	SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error)
}

func newKafkaConsumer(brokers []string, group, topic string) (Consumer, func(), error) {
	cleanup := func() {}
	config := cluster.NewConfig()
	config.Version = sarama.V1_1_0_0
	config.Consumer.Return.Errors = true
	config.Consumer.Retry.Backoff = 1 * time.Second
	config.Consumer.Offsets.Retry.Max = 5
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Metadata.Retry.Max = 5
	config.Metadata.Retry.Backoff = 1 * time.Second
	config.Group.Mode = cluster.ConsumerModeMultiplex
	if err := config.Validate(); err != nil {
		return nil, cleanup, errors.Wrap(err, "config")
	}
	client, err := cluster.NewClient(brokers, config)
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() {
		if err := client.Close(); err != nil {
			log.Println("kafka: consumer: client.Close:", err)
		}
	}
	consumer, err := cluster.NewConsumerFromClient(client, group, []string{topic})
	if err != nil {
		return nil, cleanup, errors.Wrapf(err, "consumer (brokers: %s, group: %s, topic: %s)", strings.Join(brokers, ", "), group, topic)
	}
	go func() {
		// TODO: where is the right place for this work?
		for err := range consumer.Errors() {
			log.Println("kafka: consumer:", err)
		}
	}()
	return consumer, cleanup, nil
}

func newKafkaProducer(brokers []string) (Producer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V1_1_0_0
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "config")
	}
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, errors.Wrapf(err, "producer (brokers: %s)", strings.Join(brokers, ", "))
	}
	return producer, nil
}

type mediaChunkMessage struct {
	Type          messageType `json:"type"`
	TimestampUTC  int64       `json:"timestampUTC"`
	MIMEType      string      `json:"mimeType"`
	TaskID        string      `json:"taskId"`
	TDOID         string      `json:"tdoId"`
	JobID         string      `json:"jobId"`
	ChunkIndex    int         `json:"chunkIndex"`
	StartOffsetMS int         `json:"startOffsetMs"`
	EndOffsetMS   int         `json:"endOffsetMs"`
	Width         int         `json:"width"`
	Height        int         `json:"height"`
	CacheURI      string      `json:"cacheURI"`
	Content       string      `json:"content"`
	TaskPayload   payload     `json:"taskPayload"`
	ChunkUUID     string      `json:"chunkUUID"`
}

// messageType is an enum type for edge message types.
type messageType string

// chunkStatus is an enum type for chunk statuses.
type chunkStatus string

const (
	messageTypeChunkProcessedStatus messageType = "chunk_processed_status"
	messageTypeMediaChunk           messageType = "media_chunk"
	messageTypeEngineOutput         messageType = "engine_output"
)

const (
	// chunkStatusSuccess is status to report when input chunk processed successfully
	chunkStatusSuccess chunkStatus = "SUCCESS"
	// chunkStatusError is status to report when input chunk was processed with error
	chunkStatusError chunkStatus = "ERROR"
	// chunkStatusIgnored is status to report when input chunk was ignored and not attempted processing (i.e. not of the expected type)
	chunkStatusIgnored chunkStatus = "IGNORED"
	// chunkStatusProcessing is status to report when input chunk is still being processed.  When this is the case engine needs
	// to keep sending this status every 90 seconds until either a SUCCESS or ERROR is reported.
	//chunkStatusProcessing chunkStatus = "PROCESSING"
)

// chunkProcessedStatus - processing status of a chunk by stateless engines/conductors
type chunkProcessedStatus struct {
	Type         messageType `json:"type,omitempty"`         // always messageTypeChunkProcessedStatus
	TimestampUTC int64       `json:"timestampUTC,omitempty"` // milliseconds since epoch
	TaskID       string      `json:"taskId,omitempty"`       // Task ID the chunk belongs to
	ChunkUUID    string      `json:"chunkUUID,omitempty"`    // UUID of chunk for which status is being reported
	Status       chunkStatus `json:"status,omitempty"`       // Processed status
	ErrorMsg     string      `json:"errorMsg,omitempty"`     // Optional error message in case of ERROR status
	InfoMsg      string      `json:"infoMsg,omitempty"`      // Optional message for anything engine wishes to report
}

type engineOutputMessage struct {
	Type          messageType `json:"type"`
	TimestampUTC  int64       `json:"timestampUTC"`
	OuputType     string      `json:"ouputType"`
	MIMEType      string      `json:"mimeType"`
	TaskID        string      `json:"taskId"`
	TDOID         string      `json:"tdoId"`
	JobID         string      `json:"jobId"`
	StartOffsetMS int         `json:"startOffsetMs"`
	EndOffsetMS   int         `json:"endOffsetMs"`
	Content       string      `json:"content,omitempty"`
	Rev           int64       `json:"rev"`
	TaskPayload   payload     `json:"taskPayload"`
	ChunkUUID     string      `json:"chunkUUID"`
}

type engineOutput struct {
	// SourceEngineID   string         `json:"sourceEngineId,omitempty"`
	// SourceEngineName string         `json:"sourceEngineName,omitempty"`
	// TaskPayload      payload        `json:"taskPayload,omitempty"`
	// TaskID           string         `json:"taskId"`
	// EntityID         string         `json:"entityId,omitempty"`
	// LibraryID        string         `json:"libraryId"`
	Series []seriesObject `json:"series"`
}

type seriesObject struct {
	Start     int    `json:"startTimeMs"`
	End       int    `json:"stopTimeMs"`
	EntityID  string `json:"entityId"`
	LibraryID string `json:"libraryId"`
	Object    object `json:"object"`
}

type object struct {
	Label        string   `json:"label"`
	ObjectType   string   `json:"type"`
	URI          string   `json:"uri"`
	EntityID     string   `json:"entityId,omitempty"`
	LibraryID    string   `json:"libraryId,omitempty"`
	Confidence   float64  `json:"confidence"`
	BoundingPoly []coords `json:"boundingPoly"`
}

type coords struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type payload struct {
	ApplicationID        string `json:"applicationId"`
	JobID                string `json:"jobId"`
	TaskID               string `json:"taskId"`
	RecordingID          string `json:"recordingId"`
	Token                string `json:"token"`
	Mode                 string `json:"mode,omitempty"`
	LibraryID            string `json:"libraryId"`
	LibraryEngineModelID string `json:"libraryEngineModelId"`
	StartFromEntity      string `json:"startFromEntity"`
	VeritoneAPIBaseURL   string `json:"veritoneApiBaseUrl"`
}
