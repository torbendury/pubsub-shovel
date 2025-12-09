# PubSub Shovel

A Google Cloud Function that acts as a "shovel" to transfer PubSub messages from one subscription to another topic asynchronously.

## Features

- Transfer a specified number of messages from a subscription to a topic
- Option to transfer all available messages
- Asynchronous processing for high performance
- Concurrent message handling for speed
- Proper error handling and logging
- CORS support for web applications

## API

### Endpoint
`POST /ShovelMessages` (or your function trigger URL)

### Request Payload

```json
{
  "numMessages": 100,                              // Optional: Maximum number of messages (required if allMessages is false)
  "allMessages": false,                           // Optional: Process all available messages (default: false)
  "sourceSubscription": "projects/my-project/subscriptions/source-sub",  // Required: Source subscription FQDN
  "targetTopic": "projects/my-project/topics/target-topic"               // Required: Target topic FQDN
}
```

### Parameters

- **numMessages** (int, optional): Maximum number of messages to process. Required when `allMessages` is false.
- **allMessages** (bool, optional): When true, processes all available messages in the subscription. Cannot be used with `numMessages`.
- **sourceSubscription** (string, required): Fully qualified domain name of the source subscription in format `projects/PROJECT_ID/subscriptions/SUBSCRIPTION_NAME`.
- **targetTopic** (string, required): Fully qualified domain name of the target topic in format `projects/PROJECT_ID/topics/TOPIC_NAME`.

### Response

```json
{
  "status": "accepted",
  "message": "Message shoveling started asynchronously",
  "requestId": "shovel-1701234567890"
}
```

### Error Response

```json
{
  "status": "error",
  "message": "Error description"
}
```

## Usage Examples

### Transfer 50 messages

```bash
curl -X POST https://YOUR_FUNCTION_URL \
  -H "Content-Type: application/json" \
  -d '{
    "numMessages": 50,
    "sourceSubscription": "projects/my-project/subscriptions/source-subscription",
    "targetTopic": "projects/my-project/topics/target-topic"
  }'
```

### Transfer all available messages

```bash
curl -X POST https://YOUR_FUNCTION_URL \
  -H "Content-Type: application/json" \
  -d '{
    "allMessages": true,
    "sourceSubscription": "projects/my-project/subscriptions/source-subscription",
    "targetTopic": "projects/my-project/topics/target-topic"
  }'
```

### JavaScript/Web Example

```javascript
const response = await fetch('https://YOUR_FUNCTION_URL', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    numMessages: 100,
    sourceSubscription: 'projects/my-project/subscriptions/source-subscription',
    targetTopic: 'projects/my-project/topics/target-topic'
  })
});

const result = await response.json();
console.log('Request ID:', result.requestId);
```

## Deployment

### Local Development

1. Install dependencies:
```bash
go mod download
```

2. Run locally:
```bash
go run main.go
```

3. Test with curl:
```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"numMessages": 10, "sourceSubscription": "projects/test/subscriptions/test-sub", "targetTopic": "projects/test/topics/test-topic"}'
```

### Google Cloud Functions

1. Deploy using gcloud:
```bash
gcloud functions deploy pubsub-shovel \
  --runtime go121 \
  --trigger-http \
  --entry-point ShovelMessages \
  --allow-unauthenticated
```

2. Or use the Cloud Console to deploy from source.

### Environment Requirements

- Google Cloud Project with PubSub API enabled
- Appropriate IAM permissions:
  - `pubsub.subscriber` on source subscription
  - `pubsub.publisher` on target topic
  - `pubsub.viewer` for topic existence checks

## Configuration

The function uses the default Google Cloud credentials. When running locally, ensure you have authenticated with:

```bash
gcloud auth application-default login
```

## Logging

The function provides detailed logging including:
- Request validation results
- Processing progress updates
- Error messages
- Completion summaries

View logs in Cloud Logging:
```bash
gcloud functions logs read pubsub-shovel
```

## Performance Considerations

- Processes up to 10 messages concurrently
- Maximum of 100 outstanding messages at a time
- 10-minute timeout for message processing
- Asynchronous publishing for better throughput

## Error Handling

- Validates all input parameters before processing
- Checks target topic existence before starting
- Acknowledges source messages only after successful republishing
- Provides detailed error messages in responses and logs

## Security

- CORS enabled for web applications
- Input validation on all parameters
- Uses Google Cloud IAM for authentication and authorization
- No sensitive data stored in function code
