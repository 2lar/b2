# 005 — Harden Event System & Outbox

## Priority: HIGH
## Effort: Medium
## Files: 3-4 files modified

---

## Issue 1: Event Outbox Uses Full Table Scan

**File:** `backend/infrastructure/persistence/dynamodb/event_store.go`
**Lines:** 517-550

**Bug:** `GetPendingEvents()` uses a DynamoDB Scan with FilterExpression
to find events with `PublishStatus = "pending"`. This reads every item in
the table, which is O(n) on total table size — not just events.

At scale (10k+ nodes, edges, events), this becomes a performance bottleneck
and costs real money in DynamoDB read capacity.

**Fix: Add a GSI for pending events**

### Part A: CDK — Add GSI
**File:** `infra/lib/stacks/database-stack.ts`

```typescript
table.addGlobalSecondaryIndex({
    indexName: 'PendingEventsIndex',
    partitionKey: { name: 'PublishStatus', type: AttributeType.STRING },
    sortKey: { name: 'CreatedAt', type: AttributeType.STRING },
    projectionType: ProjectionType.ALL,
});
```

### Part B: Change Scan to Query
**File:** `backend/infrastructure/persistence/dynamodb/event_store.go`

```go
func (es *EventStore) GetPendingEvents(ctx context.Context, limit int32) ([]EventRecord, error) {
    input := &dynamodb.QueryInput{
        TableName:              aws.String(es.tableName),
        IndexName:              aws.String("PendingEventsIndex"),
        KeyConditionExpression: aws.String("PublishStatus = :status"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":status": &types.AttributeValueMemberS{Value: "pending"},
        },
        Limit:            aws.Int32(limit),
        ScanIndexForward: aws.Bool(true), // Oldest first
    }
    // ...
}
```

**Effort:** Medium — GSI deployment + code change

---

## Issue 2: No Outbox Processor

**Bug:** Events are saved with `PublishStatus = "pending"` in the event store.
The UnitOfWork Commit method has a comment saying "OutboxProcessor will handle
publishing asynchronously" — but no OutboxProcessor exists in the codebase.

Events saved as pending will stay pending forever unless something processes them.

**Fix: Create an Outbox Processor**

**New file:** `backend/infrastructure/messaging/outbox/processor.go`

```go
type OutboxProcessor struct {
    eventStore    ports.EventStore
    eventPublisher ports.EventPublisher
    logger        *zap.Logger
    pollInterval  time.Duration
    batchSize     int32
}

func (p *OutboxProcessor) Start(ctx context.Context) {
    ticker := time.NewTicker(p.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.processBatch(ctx)
        }
    }
}

func (p *OutboxProcessor) processBatch(ctx context.Context) {
    events, err := p.eventStore.GetPendingEvents(ctx, p.batchSize)
    if err != nil {
        p.logger.Error("Failed to get pending events", zap.Error(err))
        return
    }

    for _, record := range events {
        event, err := record.ToDomainEvent()
        if err != nil {
            p.eventStore.MarkEventAsFailed(ctx, record.EventID, err.Error())
            continue
        }

        if err := p.eventPublisher.Publish(ctx, event); err != nil {
            p.eventStore.MarkEventAsFailed(ctx, record.EventID, err.Error())
            continue
        }

        p.eventStore.MarkEventAsPublished(ctx, record.EventID)
    }
}
```

**Deployment options:**
- Run as a goroutine in the API Lambda (piggyback on existing process)
- Run as a scheduled Lambda (CloudWatch Events, every 1 minute)
- Run in the worker Lambda (already exists as background processor)

**Recommended:** Add to the existing worker Lambda (`backend/cmd/worker/main.go`).

**Effort:** Medium — new file + wiring into worker

---

## Issue 3: No Dead Letter / Max Retry for Failed Events

**File:** `backend/infrastructure/persistence/dynamodb/event_store.go`
**Lines:** 579-610

**Current:** `MarkEventAsFailed` increments `PublishAttempts` and caps at 3.
After 3 failures, status changes to `"failed"`. But:
- No alerting when events permanently fail
- No mechanism to investigate or replay failed events
- Failed events accumulate in the table forever

**Fix:**

### Part A: Add CloudWatch metric for failed events
```go
func (es *EventStore) MarkEventAsFailed(ctx context.Context, eventID string, reason string) error {
    // ... existing logic ...

    if attempts >= maxAttempts {
        es.logger.Error("Event permanently failed after max retries",
            zap.String("eventID", eventID),
            zap.String("reason", reason),
            zap.Int("attempts", attempts),
        )
        // Emit metric for alerting
        es.metrics.IncrementCounter("events.permanently_failed", 1)
    }
    // ...
}
```

### Part B: Add event replay capability
```go
func (es *EventStore) ReplayFailedEvents(ctx context.Context, limit int32) (int, error) {
    // Query failed events
    // Reset PublishStatus to "pending" and PublishAttempts to 0
    // Return count of events replayed
}
```

### Part C: TTL cleanup for old published events
Events that are successfully published don't need to stay in the table.
Add a DynamoDB TTL attribute:
```go
// When marking as published:
item["TTL"] = &types.AttributeValueMemberN{
    Value: fmt.Sprintf("%d", time.Now().Add(7*24*time.Hour).Unix()),
}
```

**Effort:** Small-Medium

---

## Issue 4: Event Deduplication

**File:** `backend/infrastructure/persistence/dynamodb/event_store.go`
**Lines:** 82-132, 315

**Bug:** EventID is always a new UUID. If a saga step retries, the same
domain event gets saved twice with different IDs. Downstream handlers
process it twice.

**Fix:** Use a deterministic event ID based on aggregate ID + event type + version:
```go
eventID := fmt.Sprintf("%s:%s:%d", aggregateID, eventType, aggregateVersion)
```

And add a conditional write:
```go
ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)")
```

This ensures idempotent event saving — retries are no-ops.

**Effort:** Small — ~10 lines changed

---

## Verification

- [ ] Create 100 nodes rapidly → all events eventually published (check PublishStatus)
- [ ] Kill API mid-transaction → pending events picked up by outbox processor
- [ ] Simulate EventBridge failure → events retry 3 times, then marked failed
- [ ] Check CloudWatch → permanent failures emit metric
- [ ] Published events cleaned up after 7 days (TTL)
