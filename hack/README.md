# PubSub Shovel Test Scripts

This directory contains test scripts to help you validate the PubSub Shovel functionality. These scripts allow you to generate test messages and trigger the shovel operation for testing purposes.

## Scripts Overview

- **`generate-test-messages.sh`** - Generates test messages in a PubSub topic
- **`test-shovel.sh`** - Triggers the shovel operation and validates results

## Prerequisites

### 1. Google Cloud CLI Setup

```bash
# Install gcloud CLI (if not already installed)
curl https://sdk.cloud.google.com | bash
exec -l $SHELL

# Authenticate with Google Cloud
gcloud auth login

# Set your default project
gcloud config set project YOUR_PROJECT_ID
```

### 2. Required GCP Resources

You need the following PubSub resources in your GCP project:

```bash
# Set your project ID
export PROJECT_ID="your-gcp-project-id"

# Create source topic and subscription
gcloud pubsub topics create shoveltest.v1
gcloud pubsub subscriptions create shoveltest.v1-sub --topic=shoveltest.v1

# Create target topic for shoveled messages
gcloud pubsub topics create shoveltarget.v1

# Optional: Create subscription on target topic to verify results
gcloud pubsub subscriptions create shoveltarget.v1-sub --topic=shoveltarget.v1
```

### 3. Deploy PubSub Shovel Function

Make sure you have deployed the PubSub Shovel function. See the main README for deployment instructions.

## Configuration

Both scripts require your GCP Project ID to be configured. You have two options:

### Option 1: Environment Variable (Recommended)

```bash
export PROJECT_ID="your-gcp-project-id"
```

### Option 2: Edit Scripts Directly

Edit the `PROJECT_ID` variable in each script:

```bash
PROJECT_ID="your-gcp-project-id"  # Replace with your actual project ID
```

## Usage Examples

### 1. Generate Test Messages

Generate 1000 test messages in the source topic:

```bash
# Make sure scripts are executable
chmod +x generate-test-messages.sh

# Set your project ID
export PROJECT_ID="your-gcp-project-id"

# Generate test messages
./generate-test-messages.sh
```

**Output:**

```text
🚀 PubSub Test Message Generator
=================================
Project ID: your-gcp-project-id
Topic: shoveltest.v1
Number of messages: 1000

📨 Generating 1000 messages with controlled parallel processing...
⚡ Parallel processing within batches for optimal balance!

📦 Processing batch 1 (50 messages)...
✅ Batch 1 completed (50 messages)
Progress: 50/1000 (5%)

📦 Processing batch 2 (50 messages)...
✅ Batch 2 completed (50 messages)
Progress: 100/1000 (10%)

...

🎉 SUCCESS! Generated 1000 test messages using controlled parallel processing
```

### 2. Test Shovel Operation

#### Local Testing (Function running on localhost:8080)

```bash
# Make sure scripts are executable
chmod +x test-shovel.sh

# Set your project ID
export PROJECT_ID="your-gcp-project-id"

# Test with local function
./test-shovel.sh
```

#### Production Testing (Deployed Cloud Function)

```bash
# Test with deployed function URL
./test-shovel.sh "https://your-region-your-project.cloudfunctions.net/ShovelMessages"
```

#### Custom Test Scenarios

```bash
# Test shoveling 100 messages (default)
./test-shovel.sh "https://your-function-url.com" 100

# Test shoveling 500 messages
./test-shovel.sh "https://your-function-url.com" 500

# Test shoveling all available messages
./test-shovel.sh "https://your-function-url.com" all
```

**Output:**

```text
🔧 PubSub Shovel Test Script
============================
Function URL: https://your-function-url.com
Source Subscription: projects/your-project/subscriptions/shoveltest.v1-sub
Target Topic: projects/your-project/topics/shoveltarget.v1

📊 Pre-test Status Check
========================
Source messages available: 1000
Target messages before: 0

🚀 Triggering shovel operation (100 messages)...

📤 Request sent successfully!
Response: {"status":"accepted","message":"Message shoveling started asynchronously"}

⏳ Waiting for shovel operation to complete...

📊 Post-test Status Check
=========================
Source messages remaining: 900
Target messages after: 100
Messages successfully shoveled: 100

✅ Test completed successfully!
```

## Advanced Usage

### Batch Testing Workflow

Complete testing workflow from setup to validation:

```bash
# 1. Set up environment
export PROJECT_ID="your-gcp-project-id"

# 2. Generate test data
./generate-test-messages.sh

# 3. Test shoveling small batch
./test-shovel.sh "https://your-function-url.com" 50

# 4. Test shoveling larger batch
./test-shovel.sh "https://your-function-url.com" 200

# 5. Test shoveling all remaining messages
./test-shovel.sh "https://your-function-url.com" all
```

### Performance Testing

Test shovel performance with different message volumes:

```bash
# Generate large test dataset
export PROJECT_ID="your-gcp-project-id"
./generate-test-messages.sh

# Test different batch sizes
./test-shovel.sh "https://your-function-url.com" 10
./test-shovel.sh "https://your-function-url.com" 50
./test-shovel.sh "https://your-function-url.com" 100
./test-shovel.sh "https://your-function-url.com" 500
```

### Manual Verification

Verify results manually using gcloud CLI:

```bash
# Check messages in source subscription
gcloud pubsub subscriptions pull shoveltest.v1-sub --limit=5 --auto-ack

# Check messages in target topic (via subscription)
gcloud pubsub subscriptions pull shoveltarget.v1-sub --limit=5 --auto-ack

# Get subscription details
gcloud pubsub subscriptions describe shoveltest.v1-sub
gcloud pubsub subscriptions describe shoveltarget.v1-sub
```

## Script Configuration

### generate-test-messages.sh

| Variable       | Default               | Description                         |
|----------------|-----------------------|-------------------------------------|
| `PROJECT_ID`   | `your-gcp-project-id` | Your GCP project ID                 |
| `TOPIC_NAME`   | `shoveltest.v1`       | Source topic name                   |
| `NUM_MESSAGES` | `1000`                | Number of test messages to generate |
| `BATCH_SIZE`   | `50`                  | Messages per parallel batch         |

### test-shovel.sh

| Variable               | Default                 | Description                            |
|------------------------|-------------------------|----------------------------------------|
| `PROJECT_ID`           | `your-gcp-project-id`   | Your GCP project ID                    |
| `SOURCE_SUBSCRIPTION`  | `shoveltest.v1-sub`     | Source subscription name               |
| `TARGET_TOPIC`         | `shoveltarget.v1`       | Target topic name                      |
| `DEFAULT_FUNCTION_URL` | `http://localhost:8080` | Default function URL for local testing |

## Troubleshooting

### Common Issues

1. **Authentication Error**

   ```bash
   gcloud auth login
   gcloud config set project YOUR_PROJECT_ID
   ```

2. **Permission Errors**

   ```bash
   # Make scripts executable
   chmod +x *.sh
   ```

3. **Topic/Subscription Not Found**

   ```bash
   # Create required resources
   gcloud pubsub topics create shoveltest.v1
   gcloud pubsub subscriptions create shoveltest.v1-sub --topic=shoveltest.v1
   gcloud pubsub topics create shoveltarget.v1
   ```

4. **Function URL Issues**

   ```bash
   # Get your deployed function URL
   gcloud functions describe ShovelMessages --region=YOUR_REGION --format="value(httpsTrigger.url)"
   ```

### Debugging Tips

- Check function logs: `gcloud functions logs read ShovelMessages`
- Verify PubSub quotas and limits in GCP Console
- Use `--verbose` flag with gcloud commands for detailed output
- Monitor message counts before and after operations

## Performance Notes

- **Message Generation**: Uses parallel processing within batches for optimal performance
- **System Load**: Controlled to prevent overwhelming your system (max 50 concurrent processes)
- **Network**: Performance depends on network latency to Google Cloud APIs
- **Limits**: Respects Google Cloud PubSub API rate limits

## Contributing

When modifying these scripts:

1. Keep personal information out of the code (use environment variables)
2. Maintain the error handling and validation logic
3. Update this README if you add new features or change behavior
4. Test with both local and deployed functions
