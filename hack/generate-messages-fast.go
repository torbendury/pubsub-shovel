package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
)

// Config holds the configuration for message generation
type Config struct {
	ProjectID   string
	TopicName   string
	NumMessages int
	Concurrency int
	BatchSize   int
}

// TestMessage represents the structure of a test message
type TestMessage struct {
	MessageID string                 `json:"messageId"`
	Timestamp string                 `json:"timestamp"`
	Data      string                 `json:"data"`
	Source    string                 `json:"source"`
	Version   string                 `json:"version"`
	Metadata  map[string]interface{} `json:"metadata"`
}

func main() {
	// Parse command line flags
	config := Config{}
	flag.StringVar(&config.ProjectID, "project", os.Getenv("PROJECT_ID"), "Google Cloud Project ID (or set PROJECT_ID env var)")
	flag.StringVar(&config.TopicName, "topic", "shoveltest.v1", "PubSub topic name")
	flag.IntVar(&config.NumMessages, "count", 1000, "Number of messages to generate")
	flag.IntVar(&config.Concurrency, "concurrency", 10, "Number of concurrent publishers")
	flag.IntVar(&config.BatchSize, "batch", 100, "Messages per batch")
	flag.Parse()

	// Validate configuration
	if config.ProjectID == "" || config.ProjectID == "your-gcp-project-id" {
		log.Fatal("❌ Error: Please set your PROJECT_ID environment variable or use -project flag")
	}

	fmt.Printf("🚀 Fast PubSub Test Message Generator\n")
	fmt.Printf("=====================================\n")
	fmt.Printf("Project ID: %s\n", config.ProjectID)
	fmt.Printf("Topic: %s\n", config.TopicName)
	fmt.Printf("Messages: %d\n", config.NumMessages)
	fmt.Printf("Concurrency: %d\n", config.Concurrency)
	fmt.Printf("Batch Size: %d\n", config.BatchSize)
	fmt.Println()

	// Create context and PubSub client
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, config.ProjectID)
	if err != nil {
		log.Fatalf("❌ Failed to create PubSub client: %v", err)
	}
	defer client.Close()

	// Get topic
	topic := client.Topic(config.TopicName)
	exists, err := topic.Exists(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to check if topic exists: %v", err)
	}
	if !exists {
		log.Fatalf("❌ Topic %s does not exist. Create it with: gcloud pubsub topics create %s", config.TopicName, config.TopicName)
	}

	fmt.Printf("✅ Topic verification successful\n")
	fmt.Printf("📨 Generating %d messages with %d concurrent workers...\n\n", config.NumMessages, config.Concurrency)

	// Configure topic settings for better performance
	topic.PublishSettings = pubsub.PublishSettings{
		ByteThreshold:     5000,                  // Batch when we have 5KB of data
		CountThreshold:    100,                   // Batch when we have 100 messages
		DelayThreshold:    10 * time.Millisecond, // Batch after 10ms delay
		NumGoroutines:     config.Concurrency,
		BufferedByteLimit: 10e6, // 10MB buffer
	}

	// Start time measurement
	startTime := time.Now()

	// Generate and publish messages
	err = generateMessages(ctx, topic, &config)
	if err != nil {
		log.Fatalf("❌ Failed to generate messages: %v", err)
	}

	// Calculate statistics
	duration := time.Since(startTime)
	messagesPerSecond := float64(config.NumMessages) / duration.Seconds()

	fmt.Printf("\n🎉 SUCCESS! Generated %d messages\n", config.NumMessages)
	fmt.Printf("📊 Performance Statistics:\n")
	fmt.Printf("  • Total time: %v\n", duration)
	fmt.Printf("  • Messages per second: %.1f\n", messagesPerSecond)
	fmt.Printf("  • Average latency: %v per message\n", duration/time.Duration(config.NumMessages))
	fmt.Println()
	fmt.Printf("💡 Next steps:\n")
	fmt.Printf("  1. Verify messages: gcloud pubsub subscriptions pull %s-sub --limit=5\n", config.TopicName)
	fmt.Printf("  2. Test shovel function with your test script\n")
	fmt.Println()
}

func generateMessages(ctx context.Context, topic *pubsub.Topic, config *Config) error {
	var wg sync.WaitGroup
	var published int64
	var mu sync.Mutex

	// Progress tracking
	progressChan := make(chan int, config.NumMessages)
	go trackProgress(progressChan, config.NumMessages)

	// Create batches of work
	batchCount := (config.NumMessages + config.BatchSize - 1) / config.BatchSize
	workChan := make(chan workBatch, batchCount)

	// Generate work batches
	for i := 0; i < batchCount; i++ {
		start := i*config.BatchSize + 1
		end := start + config.BatchSize - 1
		if end > config.NumMessages {
			end = config.NumMessages
		}
		workChan <- workBatch{start: start, end: end}
	}
	close(workChan)

	// Start worker goroutines
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for batch := range workChan {
				batchPublished := publishBatch(ctx, topic, batch, config.ProjectID)

				mu.Lock()
				published += int64(batchPublished)
				mu.Unlock()

				// Report progress for each message in the batch
				for j := 0; j < batchPublished; j++ {
					progressChan <- 1
				}
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(progressChan)

	if published != int64(config.NumMessages) {
		return fmt.Errorf("expected to publish %d messages, but published %d", config.NumMessages, published)
	}

	return nil
}

type workBatch struct {
	start, end int
}

func publishBatch(ctx context.Context, topic *pubsub.Topic, batch workBatch, projectID string) int {
	var results []*pubsub.PublishResult
	batchID := fmt.Sprintf("batch-%d", time.Now().UnixNano()/1000000)

	// Publish all messages in the batch
	for msgNum := batch.start; msgNum <= batch.end; msgNum++ {
		message := createTestMessage(msgNum, batchID, projectID)

		// Convert message to JSON
		messageJSON, err := json.Marshal(message)
		if err != nil {
			log.Printf("⚠️ Failed to marshal message %d: %v", msgNum, err)
			continue
		}

		// Publish message (this is async and batched by the client)
		result := topic.Publish(ctx, &pubsub.Message{
			Data: messageJSON,
			Attributes: map[string]string{
				"messageType":   "test",
				"batchId":       batchID,
				"messageNumber": fmt.Sprintf("%d", msgNum),
				"generator":     "go-fast-generator",
			},
		})

		results = append(results, result)
	}

	// Wait for all publishes in this batch to complete
	published := 0
	for i, result := range results {
		msgNum := batch.start + i
		_, err := result.Get(ctx)
		if err != nil {
			log.Printf("⚠️ Failed to publish message %d: %v", msgNum, err)
		} else {
			published++
		}
	}

	return published
}

func createTestMessage(msgNum int, batchID, projectID string) TestMessage {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	return TestMessage{
		MessageID: fmt.Sprintf("test-msg-%d-%d", msgNum, time.Now().UnixNano()/1000000),
		Timestamp: timestamp,
		Data:      fmt.Sprintf("Test message %d generated by Go fast generator", msgNum),
		Source:    "go-fast-generator",
		Version:   "1.0",
		Metadata: map[string]interface{}{
			"batchId":       batchID,
			"projectId":     projectID,
			"environment":   "testing",
			"messageNumber": msgNum,
			"generatedAt":   timestamp,
			"randomValue":   time.Now().UnixNano() % 10000,
		},
	}
}

func trackProgress(progressChan <-chan int, total int) {
	processed := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastUpdate := time.Now()

	for {
		select {
		case <-progressChan:
			processed++
			if processed >= total {
				fmt.Printf("\r✅ Progress: %d/%d (100%%) - Complete!         \n", processed, total)
				return
			}
		case <-ticker.C:
			if processed > 0 && time.Since(lastUpdate) > 500*time.Millisecond {
				percentage := (processed * 100) / total
				fmt.Printf("\r⚡ Progress: %d/%d (%d%%)...", processed, total, percentage)
				lastUpdate = time.Now()
			}
		}
	}
}
