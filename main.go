package main

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

func init() {
	functions.HTTP("ShovelMessages", ShovelMessages)
}

// ShovelMessages is the HTTP Cloud Function entry point
func ShovelMessages(w http.ResponseWriter, r *http.Request) {
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

	if r.Method != "POST" {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShovelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		respondWithError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateRequest(&req); err != nil {
		log.Printf("Validation error: %v", err)
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate request ID for tracking
	requestID := fmt.Sprintf("shovel-%d", time.Now().UnixNano())

	// Start asynchronous processing
	go func() {
		ctx := context.Background()
		count, err := processMessages(ctx, &req, requestID)
		if err != nil {
			log.Printf("Request %s failed: %v", requestID, err)
		} else {
			log.Printf("Request %s completed successfully, processed %d messages", requestID, count)
		}
	}()

	// Return immediate response
	response := ShovelResponse{
		Status:    "accepted",
		Message:   "Message shoveling started asynchronously",
		RequestID: requestID,
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
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
		return fmt.Errorf("numMessages must be positive when allMessages is false")
	}
	if req.AllMessages && req.NumMessages > 0 {
		return fmt.Errorf("cannot specify both allMessages=true and numMessages")
	}
	return nil
}

// processMessages handles the actual message processing
func processMessages(ctx context.Context, req *ShovelRequest, requestID string) (int, error) {
	// Create PubSub client
	client, err := pubsub.NewClient(ctx, extractProjectID(req.SourceSubscription))
	if err != nil {
		return 0, fmt.Errorf("failed to create pubsub client: %v", err)
	}
	defer client.Close()

	// Get source subscription
	sourceSubName := extractResourceName(req.SourceSubscription)
	sourceSub := client.Subscription(sourceSubName)

	// Get target topic
	targetTopicName := extractResourceName(req.TargetTopic)
	targetTopic := client.Topic(targetTopicName)

	// Check if target topic exists
	exists, err := targetTopic.Exists(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to check target topic existence: %v", err)
	}
	if !exists {
		return 0, fmt.Errorf("target topic %s does not exist", req.TargetTopic)
	}

	log.Printf("Request %s: Starting to process messages from %s to %s",
		requestID, req.SourceSubscription, req.TargetTopic)

	var processedCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Set receive settings for better performance
	sourceSub.ReceiveSettings.Synchronous = false
	sourceSub.ReceiveSettings.NumGoroutines = 10
	sourceSub.ReceiveSettings.MaxOutstandingMessages = 100

	// Context with timeout for receiving messages
	receiveCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Channel to control when to stop receiving
	done := make(chan bool, 1)

	err = sourceSub.Receive(receiveCtx, func(ctx context.Context, msg *pubsub.Message) {
		defer msg.Ack()

		// Check if we should stop processing
		select {
		case <-done:
			return
		default:
		}

		wg.Add(1)
		go func(message *pubsub.Message) {
			defer wg.Done()

			// Republish message to target topic
			result := targetTopic.Publish(ctx, &pubsub.Message{
				Data:       message.Data,
				Attributes: message.Attributes,
			})

			// Wait for publish to complete
			if _, err := result.Get(ctx); err != nil {
				log.Printf("Request %s: Failed to publish message: %v", requestID, err)
				return
			}

			mu.Lock()
			processedCount++
			current := processedCount
			mu.Unlock()

			log.Printf("Request %s: Processed message %d", requestID, current)

			// Check if we've reached the target number
			if !req.AllMessages && current >= req.NumMessages {
				select {
				case done <- true:
				default:
				}
			}
		}(msg)

		// If we're not processing all messages and we've reached our target, stop
		if !req.AllMessages {
			mu.Lock()
			current := processedCount
			mu.Unlock()
			if current >= req.NumMessages {
				select {
				case done <- true:
				default:
				}
			}
		}
	})

	// Wait for all publishing to complete
	wg.Wait()

	if err != nil && err != context.Canceled {
		return processedCount, fmt.Errorf("error during message processing: %v", err)
	}

	log.Printf("Request %s: Completed processing %d messages", requestID, processedCount)
	return processedCount, nil
}

// extractProjectID extracts the project ID from a fully qualified resource name
func extractProjectID(fqdn string) string {
	// Expected format: projects/PROJECT_ID/subscriptions/SUBSCRIPTION_NAME
	// or projects/PROJECT_ID/topics/TOPIC_NAME
	parts := splitResourceName(fqdn)
	if len(parts) >= 2 && parts[0] == "projects" {
		return parts[1]
	}
	return ""
}

// extractResourceName extracts the resource name from a fully qualified resource name
func extractResourceName(fqdn string) string {
	// Expected format: projects/PROJECT_ID/subscriptions/SUBSCRIPTION_NAME
	// or projects/PROJECT_ID/topics/TOPIC_NAME
	parts := splitResourceName(fqdn)
	if len(parts) >= 4 {
		return parts[3]
	}
	return fqdn // fallback to original if parsing fails
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
	json.NewEncoder(w).Encode(response)
}

// main is required for local testing but not used in Cloud Functions
func main() {
	// Start the functions framework
	port := "8080"
	if p := getEnvVar("PORT"); p != "" {
		port = p
	}

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// getEnvVar is a helper to get environment variables with fallback
func getEnvVar(key string) string {
	return os.Getenv(key)
}
