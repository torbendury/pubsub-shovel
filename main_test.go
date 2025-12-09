package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShovelMessages_ValidationErrors(t *testing.T) {
	tests := []struct {
		name         string
		payload      ShovelRequest
		expectedCode int
	}{
		{
			name: "missing source subscription",
			payload: ShovelRequest{
				NumMessages: 10,
				TargetTopic: "projects/test/topics/target",
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "missing target topic",
			payload: ShovelRequest{
				NumMessages:        10,
				SourceSubscription: "projects/test/subscriptions/source",
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "negative num messages",
			payload: ShovelRequest{
				NumMessages:        -1,
				SourceSubscription: "projects/test/subscriptions/source",
				TargetTopic:        "projects/test/topics/target",
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "both allMessages and numMessages specified",
			payload: ShovelRequest{
				NumMessages:        10,
				AllMessages:        true,
				SourceSubscription: "projects/test/subscriptions/source",
				TargetTopic:        "projects/test/topics/target",
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "valid request with numMessages",
			payload: ShovelRequest{
				NumMessages:        10,
				SourceSubscription: "projects/test/subscriptions/source",
				TargetTopic:        "projects/test/topics/target",
			},
			expectedCode: http.StatusAccepted,
		},
		{
			name: "valid request with allMessages",
			payload: ShovelRequest{
				AllMessages:        true,
				SourceSubscription: "projects/test/subscriptions/source",
				TargetTopic:        "projects/test/topics/target",
			},
			expectedCode: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ShovelMessages(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, rr.Code)
			}
		})
	}
}

func TestShovelMessages_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	ShovelMessages(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestShovelMessages_OptionsRequest(t *testing.T) {
	req := httptest.NewRequest("OPTIONS", "/", nil)
	rr := httptest.NewRecorder()

	ShovelMessages(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected status code %d, got %d", http.StatusNoContent, rr.Code)
	}

	// Check CORS headers
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "POST, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type",
	}

	for header, expectedValue := range expectedHeaders {
		if rr.Header().Get(header) != expectedValue {
			t.Errorf("Expected header %s: %s, got %s", header, expectedValue, rr.Header().Get(header))
		}
	}
}

func TestExtractProjectID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "projects/my-project/subscriptions/my-sub",
			expected: "my-project",
		},
		{
			input:    "projects/another-project/topics/my-topic",
			expected: "another-project",
		},
		{
			input:    "invalid-format",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractProjectID(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExtractResourceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "projects/my-project/subscriptions/my-subscription",
			expected: "my-subscription",
		},
		{
			input:    "projects/my-project/topics/my-topic",
			expected: "my-topic",
		},
		{
			input:    "just-a-name",
			expected: "just-a-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractResourceName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
