package shovel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

func init() {
	functions.HTTP("Handler", Handler)
}

// ShovelRequest represents the HTTP request payload
type ShovelRequest struct {
	NumMessages        int    `json:"numMessages,omitempty"` // Maximum number of messages to process
	AllMessages        bool   `json:"allMessages,omitempty"` // Process all available messages
	SourceSubscription string `json:"sourceSubscription"`    // Source subscription FQDN
	TargetTopic        string `json:"targetTopic"`           // Target topic FQDN
}

// ShovelResponse represents the HTTP response
type ShovelResponse struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	ProcessedCount int    `json:"processedCount,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
}

// Handler handles the shovel HTTP requests
func Handler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Only allow POST requests
	if r.Method != "POST" {
		respondWithError(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req ShovelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateRequest(&req); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate request ID for tracking
	requestID := fmt.Sprintf("shovel-%d", time.Now().UnixNano()/1000000)
	log.Printf("Processing shovel request %s: %+v", requestID, req)

	// Start async processing
	go func() {
		ctx := context.Background()
		processedCount, err := processShovelRequest(ctx, &req)
		if err != nil {
			log.Printf("Request %s failed: %v", requestID, err)
		} else {
			log.Printf("Request %s completed successfully, processed %d messages", requestID, processedCount)
		}
	}()

	// Return immediate response
	response := ShovelResponse{
		Status:    "accepted",
		Message:   "Message shoveling started asynchronously",
		RequestID: requestID,
	}

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// validateRequest validates the incoming request
func validateRequest(req *ShovelRequest) error {
	if req.SourceSubscription == "" {
		return fmt.Errorf("sourceSubscription is required")
	}
	if req.TargetTopic == "" {
		return fmt.Errorf("targetTopic is required")
	}
	if !req.AllMessages && req.NumMessages <= 0 {
		return fmt.Errorf("numMessages must be greater than 0 when allMessages is false")
	}
	if req.AllMessages && req.NumMessages > 0 {
		return fmt.Errorf("cannot specify both allMessages=true and numMessages > 0")
	}
	return nil
}

// processShovelRequest handles the actual message shoveling
func processShovelRequest(ctx context.Context, req *ShovelRequest) (int, error) {
	// Create PubSub client
	client, err := pubsub.NewClient(ctx, extractProjectID(req.SourceSubscription))
	if err != nil {
		return 0, fmt.Errorf("failed to create pubsub client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Failed to close pubsub client: %v", err)
		}
	}()

	// Get source subscription
	sourceSubName := extractResourceName(req.SourceSubscription)
	sourceSub := client.Subscription(sourceSubName)

	// Get target topic
	targetTopicName := extractResourceName(req.TargetTopic)
	targetTopic := client.Topic(targetTopicName)

	// Check if target topic exists
	exists, err := targetTopic.Exists(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to check if target topic exists: %v", err)
	}
	if !exists {
		return 0, fmt.Errorf("target topic %s does not exist", req.TargetTopic)
	}

	// Set receive settings for better performance
	sourceSub.ReceiveSettings.Synchronous = false
	sourceSub.ReceiveSettings.NumGoroutines = 10
	sourceSub.ReceiveSettings.MaxOutstandingMessages = 100

	// Determine number of messages to process
	maxMessages := req.NumMessages
	if req.AllMessages {
		// For "all messages", we'll set a high limit and process until no more messages
		maxMessages = 10000 // Reasonable upper limit
	}

	// Process messages with proper concurrency control
	var acceptedCount int  // Messages accepted for processing
	var processedCount int // Messages successfully processed
	var mutex sync.Mutex
	done := make(chan bool)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		err = sourceSub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			mutex.Lock()
			// Check if we've already accepted enough messages
			if acceptedCount >= maxMessages {
				mutex.Unlock()
				msg.Nack()
				cancel()
				return
			}
			// Increment accepted count immediately to prevent race condition
			acceptedCount++
			// currentAccepted := acceptedCount
			mutex.Unlock()

			//log.Printf("Accepted message %d/%d for processing", currentAccepted, maxMessages)

			// Publish to target topic
			result := targetTopic.Publish(ctx, &pubsub.Message{
				Data:       msg.Data,
				Attributes: msg.Attributes,
			})

			// Wait for publish result
			go func() {
				_, publishErr := result.Get(ctx)
				if publishErr != nil {
					//log.Printf("Failed to publish message: %v", publishErr)
					msg.Nack()
					// Don't decrement acceptedCount since we want to stop at the limit
				} else {
					// Acknowledge original message
					msg.Ack()
					mutex.Lock()
					processedCount++
					//currentProcessed := processedCount
					mutex.Unlock()
					//log.Printf("Successfully processed message %d/%d (accepted: %d)", currentProcessed, maxMessages, currentAccepted)
				}
			}()
		})

		if err != nil {
			log.Printf("Receive error: %v", err)
		}
		done <- true
	}()

	// Set timeout for processing
	timeout := 5 * time.Minute
	if req.AllMessages {
		timeout = 10 * time.Minute // Longer timeout for "all messages"
	}

	select {
	case <-done:
		log.Printf("Message processing completed")
	case <-time.After(timeout):
		log.Printf("Processing timeout reached")
		cancel()
	}

	// Wait a bit for any pending operations to complete
	time.Sleep(2 * time.Second)

	mutex.Lock()
	finalAccepted := acceptedCount
	finalProcessed := processedCount
	mutex.Unlock()

	log.Printf("Shovel completed: accepted %d messages, successfully processed %d messages", finalAccepted, finalProcessed)
	return finalProcessed, nil
}

// extractProjectID extracts project ID from a resource name
func extractProjectID(resourceName string) string {
	parts := splitResourceName(resourceName)
	for i, part := range parts {
		if part == "projects" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// extractResourceName extracts the resource name from FQDN
func extractResourceName(fqdn string) string {
	parts := splitResourceName(fqdn)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// splitResourceName splits a resource name by '/'
func splitResourceName(name string) []string {
	result := []string{}
	current := ""
	for _, char := range name {
		if char == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// respondWithError sends an error response
func respondWithError(w http.ResponseWriter, message string, statusCode int) {
	response := ShovelResponse{
		Status:  "error",
		Message: message,
	}
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}

// GetEnvVar is a helper to get environment variables with fallback
func GetEnvVar(key string) string {
	return os.Getenv(key)
}
