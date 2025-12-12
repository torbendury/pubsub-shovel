#!/bin/bash

# Test script for PubSub Shovel Function
#
# Usage:
#   ./test-shovel.sh [FUNCTION_URL] [TEST_TYPE]
#
# Configuration:
#   Set PROJECT_ID environment variable: export PROJECT_ID="your-project-id"
#   Or modify the PROJECT_ID variable below
#
# Examples:
#   ./test-shovel.sh                                    # Test with localhost
#   ./test-shovel.sh https://your-function-url.com      # Test with deployed function

set -e

# Configuration for your test environment
# You can also set PROJECT_ID as an environment variable
PROJECT_ID=${PROJECT_ID:-"your-gcp-project-id"}
SOURCE_SUBSCRIPTION="projects/${PROJECT_ID}/subscriptions/shoveltest.v1-sub"
TARGET_TOPIC="projects/${PROJECT_ID}/topics/shoveltarget.v1"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default function URL (update this with your deployed function URL)
DEFAULT_FUNCTION_URL="http://localhost:8080/Handler"  # For local testing
FUNCTION_URL=${1:-$DEFAULT_FUNCTION_URL}

# Keep the URL as provided (both root and /ShovelMessages paths work locally now)
TEST_TYPE=${2:-"10"}

echo -e "${BLUE}🔧 PubSub Shovel Test Script${NC}"
echo -e "${BLUE}============================${NC}"
# Validate PROJECT_ID is set
if [ "${PROJECT_ID}" = "your-gcp-project-id" ]; then
    echo -e "${RED}❌ Error: Please set your PROJECT_ID${NC}"
    echo -e "Either set environment variable: ${YELLOW}export PROJECT_ID=\"your-actual-project-id\"${NC}"
    echo -e "Or edit the PROJECT_ID variable in this script"
    exit 1
fi

echo -e "Function URL: ${GREEN}${FUNCTION_URL}${NC}"
echo -e "Source Subscription: ${GREEN}${SOURCE_SUBSCRIPTION}${NC}"
echo -e "Target Topic: ${GREEN}${TARGET_TOPIC}${NC}"
echo ""

# Check if function is reachable
echo -e "${YELLOW}🔍 Checking function availability...${NC}"
if curl -s --max-time 5 "$FUNCTION_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Function is reachable${NC}"
elif [[ "$FUNCTION_URL" == *"localhost"* ]]; then
    echo -e "${RED}❌ Cannot reach function at ${FUNCTION_URL}${NC}"
    echo -e "${YELLOW}💡 Make sure you've started the function locally:${NC}"
    echo -e "   ${BLUE}go run main.go${NC}"
    echo ""
    echo -e "${YELLOW}Continuing with tests anyway...${NC}"
else
    echo -e "${RED}❌ Cannot reach function at ${FUNCTION_URL}${NC}"
    echo -e "${YELLOW}💡 Check your function URL and deployment status${NC}"
    echo ""
    echo -e "${YELLOW}Continuing with tests anyway...${NC}"
fi
echo ""

# Function to run curl command and display response
run_shovel_test() {
    local payload="$1"
    local description="$2"

    echo -e "${YELLOW}🚀 Testing: ${description}${NC}"
    echo -e "${BLUE}Payload:${NC}"
    echo "$payload" | jq .
    echo ""

    echo -e "${BLUE}Making request to: ${FUNCTION_URL}${NC}"

    # Make the curl request and capture both response and HTTP status
    echo -e "${BLUE}Response:${NC}"
    response=$(curl -s -w "\n%{http_code}" -X POST "$FUNCTION_URL" \
        -H "Content-Type: application/json" \
        -d "$payload")

    # Split response and status code
    response_body=$(echo "$response" | sed '$d')
    http_status=$(echo "$response" | tail -n1)

    echo -e "${BLUE}HTTP Status: ${http_status}${NC}"

    # Check if response is JSON
    if echo "$response_body" | jq . > /dev/null 2>&1; then
        echo -e "${GREEN}JSON Response:${NC}"
        echo "$response_body" | jq .

        # Extract request ID if present
        request_id=$(echo "$response_body" | jq -r '.requestId // empty')
        if [[ -n "$request_id" ]]; then
            echo -e "${GREEN}✅ Request ID: ${request_id}${NC}"
        fi
    else
        echo -e "${RED}Non-JSON Response:${NC}"
        echo "$response_body"

        # Check for common issues
        if [[ "$http_status" == "000" ]]; then
            echo -e "${RED}❌ Connection failed - is the function running?${NC}"
        elif [[ "$response_body" == *"Connection refused"* ]]; then
            echo -e "${RED}❌ Connection refused - check if function is running on ${FUNCTION_URL}${NC}"
        elif [[ "$response_body" == *"404"* ]] || [[ "$http_status" == "404" ]]; then
            echo -e "${RED}❌ Function not found - check the URL${NC}"
        fi
    fi
    echo ""
}
# Test cases based on TEST_TYPE parameter
case "$TEST_TYPE" in
    "100")
        echo -e "${YELLOW}📋 Test Case: Transfer 100 messages${NC}"
        payload='{
            "numMessages": 100,
            "sourceSubscription": "'$SOURCE_SUBSCRIPTION'",
            "targetTopic": "'$TARGET_TOPIC'"
        }'
        run_shovel_test "$payload" "Transfer 100 messages"
        ;;

    "all")
        echo -e "${YELLOW}📋 Test Case: Transfer ALL messages${NC}"
        payload='{
            "allMessages": true,
            "sourceSubscription": "'$SOURCE_SUBSCRIPTION'",
            "targetTopic": "'$TARGET_TOPIC'"
        }'
        run_shovel_test "$payload" "Transfer all available messages"
        ;;

    "50")
        echo -e "${YELLOW}📋 Test Case: Transfer 50 messages${NC}"
        payload='{
            "numMessages": 50,
            "sourceSubscription": "'$SOURCE_SUBSCRIPTION'",
            "targetTopic": "'$TARGET_TOPIC'"
        }'
        run_shovel_test "$payload" "Transfer 50 messages"
        ;;

    "500")
        echo -e "${YELLOW}📋 Test Case: Transfer 500 messages${NC}"
        payload='{
            "numMessages": 500,
            "sourceSubscription": "'$SOURCE_SUBSCRIPTION'",
            "targetTopic": "'$TARGET_TOPIC'"
        }'
        run_shovel_test "$payload" "Transfer 500 messages"
        ;;

    "validation")
        echo -e "${YELLOW}📋 Test Case: Validation errors${NC}"

        # Test missing source subscription
        payload='{"numMessages": 10, "targetTopic": "'$TARGET_TOPIC'"}'
        run_shovel_test "$payload" "Missing source subscription (should fail)"

        # Test missing target topic
        payload='{"numMessages": 10, "sourceSubscription": "'$SOURCE_SUBSCRIPTION'"}'
        run_shovel_test "$payload" "Missing target topic (should fail)"

        # Test negative messages
        payload='{"numMessages": -1, "sourceSubscription": "'$SOURCE_SUBSCRIPTION'", "targetTopic": "'$TARGET_TOPIC'"}'
        run_shovel_test "$payload" "Negative message count (should fail)"
        ;;

    *)
        echo -e "${RED}❌ Unknown test type: $TEST_TYPE${NC}"
        echo ""
        echo -e "${YELLOW}Available test types:${NC}"
        echo -e "  • ${GREEN}100${NC}        - Transfer 100 messages (default)"
        echo -e "  • ${GREEN}all${NC}        - Transfer all available messages"
        echo -e "  • ${GREEN}50${NC}         - Transfer 50 messages"
        echo -e "  • ${GREEN}500${NC}        - Transfer 500 messages"
        echo -e "  • ${GREEN}validation${NC} - Test validation errors"
        echo ""
        echo -e "${YELLOW}Usage examples:${NC}"
        echo -e "  ./test-shovel.sh                                    # Local test, 100 messages"
        echo -e "  ./test-shovel.sh https://your-function-url all      # Cloud function, all messages"
        echo -e "  ./test-shovel.sh http://localhost:8080 validation   # Local test, validation errors"
        exit 1
        ;;
esac

echo -e "${GREEN}✅ Test completed!${NC}"
