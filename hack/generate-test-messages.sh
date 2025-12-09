#!/bin/bash

# Script to create 1000 test messages in PubSub topic for shovel testing
#
# Usage:
#   ./generate-test-messages.sh
#
# Configuration:
#   Set PROJECT_ID environment variable: export PROJECT_ID="your-project-id"
#   Or modify the PROJECT_ID variable below

set -e

# Configuration - Set these values for your environment
# You can also set PROJECT_ID as an environment variable
PROJECT_ID=${PROJECT_ID:-"your-gcp-project-id"}
TOPIC_NAME="shoveltest.v1"
TOPIC_FQDN="projects/${PROJECT_ID}/topics/${TOPIC_NAME}"
NUM_MESSAGES=1000

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 PubSub Test Message Generator${NC}"
echo -e "${BLUE}=================================${NC}"
# Validate PROJECT_ID is set
if [ "${PROJECT_ID}" = "your-gcp-project-id" ]; then
    echo -e "${RED}❌ Error: Please set your PROJECT_ID${NC}"
    echo -e "Either set environment variable: ${YELLOW}export PROJECT_ID=\"your-actual-project-id\"${NC}"
    echo -e "Or edit the PROJECT_ID variable in this script"
    exit 1
fi

echo -e "Project ID: ${GREEN}${PROJECT_ID}${NC}"
echo -e "Topic: ${GREEN}${TOPIC_NAME}${NC}"
echo -e "Number of messages: ${GREEN}${NUM_MESSAGES}${NC}"
echo ""

# Check if gcloud is installed and authenticated
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ Error: gcloud CLI is not installed${NC}"
    echo "Please install gcloud CLI and try again."
    exit 1
fi

# Check if authenticated
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" | head -n 1 > /dev/null; then
    echo -e "${RED}❌ Error: Not authenticated with gcloud${NC}"
    echo "Please run: gcloud auth login"
    exit 1
fi

# Verify project exists and we have access
echo -e "${YELLOW}🔍 Verifying project access...${NC}"
if ! gcloud projects describe "${PROJECT_ID}" > /dev/null 2>&1; then
    echo -e "${RED}❌ Error: Cannot access project ${PROJECT_ID}${NC}"
    echo "Please check the project ID and your permissions."
    exit 1
fi

# Set the project
gcloud config set project "${PROJECT_ID}"

# Check if topic exists
echo -e "${YELLOW}🔍 Verifying topic exists...${NC}"
if ! gcloud pubsub topics describe "${TOPIC_NAME}" > /dev/null 2>&1; then
    echo -e "${RED}❌ Error: Topic ${TOPIC_NAME} does not exist${NC}"
    echo "Please create the topic first:"
    echo "gcloud pubsub topics create ${TOPIC_NAME}"
    exit 1
fi

echo -e "${GREEN}✅ Topic verification successful${NC}"
echo ""

# Function to generate a random message payload
generate_message() {
    local msg_id=$1
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.%3NZ")
    local random_data=$(openssl rand -hex 16)

    cat << EOF
{
  "messageId": "${msg_id}",
  "timestamp": "${timestamp}",
  "data": "Test message ${msg_id} - ${random_data}",
  "source": "shovel-test-generator",
  "version": "1.0",
  "metadata": {
    "batchId": "test-batch-$(date +%s)",
    "environment": "testing",
    "randomValue": ${RANDOM}
  }
}
EOF
}

# Controlled parallel processing within batches
publish_batch() {
    local start_num=$1
    local end_num=$2
    local batch_id=$3
    local batch_size=$((end_num - start_num + 1))

    echo -e "${YELLOW}📦 Processing batch ${batch_id} (${batch_size} messages)...${NC}"

    # Process messages in this batch with controlled parallelism
    for ((msg_num=start_num; msg_num<=end_num; msg_num++)); do
        message_data=$(generate_message $msg_num)

        # Publish message with attributes in background
        gcloud pubsub topics publish "${TOPIC_NAME}" \
            --message="${message_data}" \
            --attribute="messageType=test,batchId=batch-${batch_id},messageNumber=${msg_num}" \
            > /dev/null 2>&1 &
    done

    # Wait for all messages in this batch to complete before returning
    wait
    echo -e "${GREEN}✅ Batch ${batch_id} completed (${batch_size} messages)${NC}"
}

# Create messages with controlled parallel processing
BATCH_SIZE=50  # Smaller batches for better system responsiveness
TOTAL_BATCHES=$((NUM_MESSAGES / BATCH_SIZE))
REMAINING_MESSAGES=$((NUM_MESSAGES % BATCH_SIZE))

echo -e "${BLUE}📨 Generating ${NUM_MESSAGES} messages with controlled parallel processing...${NC}"
echo -e "${YELLOW}⚡ Parallel processing within batches for optimal balance!${NC}"
echo ""

# Progress tracking
processed=0

# Process batches sequentially, but parallelize within each batch
for ((batch=1; batch<=TOTAL_BATCHES; batch++)); do
    start_msg=$(((batch-1)*BATCH_SIZE + 1))
    end_msg=$((batch*BATCH_SIZE))

    # Process this batch (parallel within, sequential between batches)
    publish_batch $start_msg $end_msg $batch

    processed=$((batch * BATCH_SIZE))
    progress=$((processed * 100 / NUM_MESSAGES))
    echo -e "${BLUE}Progress: ${processed}/${NUM_MESSAGES} (${progress}%)${NC}"
    echo ""
done

# Process remaining messages if any
if [ $REMAINING_MESSAGES -gt 0 ]; then
    echo -e "${YELLOW}📦 Processing remaining ${REMAINING_MESSAGES} messages...${NC}"

    start_msg=$((TOTAL_BATCHES*BATCH_SIZE + 1))
    end_msg=$((TOTAL_BATCHES*BATCH_SIZE + REMAINING_MESSAGES))
    publish_batch $start_msg $end_msg "final"

    processed=$NUM_MESSAGES
    echo -e "${GREEN}✅ All messages completed. Total: ${processed}/${NUM_MESSAGES}${NC}"
fi

echo ""
echo -e "${GREEN}🎉 SUCCESS! Generated ${NUM_MESSAGES} test messages using controlled parallel processing${NC}"
echo -e "${BLUE}📊 Summary:${NC}"
echo -e "  • Project: ${PROJECT_ID}"
echo -e "  • Topic: ${TOPIC_NAME}"
echo -e "  • Messages created: ${processed}"
echo -e "  • Batch size: ${BATCH_SIZE} messages per batch"
echo -e "  • Batches processed: ${TOTAL_BATCHES}"
echo ""
echo -e "${YELLOW}💡 Next steps:${NC}"
echo -e "  1. Verify messages in subscription: gcloud pubsub subscriptions pull shoveltest.v1-sub --limit=5"
echo -e "  2. Use the curl command to trigger shoveling"
echo -e "  3. Check target topic for transferred messages"
echo ""
echo -e "${GREEN}✅ Script completed successfully!${NC}"
