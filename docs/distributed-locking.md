# Distributed Locking (DynamoDB)

This app uses a lightweight, DynamoDB‑backed distributed lock to serialize critical sections across Lambdas (for example, ensuring a user's default graph is created at most once).

## Why
- Prevent race conditions in multi‑step flows that cannot be expressed as a single conditional write.
- Keep lock scope narrow (e.g., per user or per graph) so unrelated work does not contend.
- Allow safe takeover when a holder crashes (lock expiry + TTL).

## Scope and Naming
- Each lock is identified by a caller‑chosen `resourceName`.
- The DynamoDB key is `PK = LOCK#<resourceName>`, `SK = LOCK`.
- Example used here: `default_graph_creation_<userID>` → one lock per user's default graph creation.

## Data Model (Lock Record)
- Attributes (stored as one item):
  - `PK`, `SK`
  - `LockID` (unique per acquisition), `Owner` (caller identifier)
  - `AcquiredAt`, `ExpiresAt` (RFC3339 strings)
  - `TTL` (Unix epoch, for DynamoDB TTL cleanup)

File reference: backend/infrastructure/persistence/dynamodb/distributed_lock.go:24

## Acquire Flow (Conditional Put)
```
Client           DistributedLock            DynamoDB
  |  TryAcquire(resource, owner, duration, timeout)
  |----------------------------------------------->
  |   Build item (PK=LOCK#resource, SK=LOCK,
  |    LockID, Owner, AcquiredAt, ExpiresAt, TTL)
  |   PutItem with Condition:
  |     attribute_not_exists(PK) OR ExpiresAt < :now
  |----------------------------------------------->
  |                                (OK)    |  (ConditionalCheckFailed)
  |<-------------------------------------- | <------------------------
  |  return Lock{lockID,...}               | retry with backoff until timeout
  |
```

Code:
- Acquire: backend/infrastructure/persistence/dynamodb/distributed_lock.go:43
- Condition: backend/infrastructure/persistence/dynamodb/distributed_lock.go:74
- Retry/backoff loop: backend/infrastructure/persistence/dynamodb/distributed_lock.go:109

## Release Flow (Conditional Delete)
```
Client           DistributedLock            DynamoDB
  |  Release(resource, lockID, owner)
  |----------------------------------------------->
  |   DeleteItem with Condition:
  |     LockID = :lockId AND Owner = :owner
  |----------------------------------------------->
  |                                 (OK or already gone)
  |<----------------------------------------------
```

Code:
- Release: backend/infrastructure/persistence/dynamodb/distributed_lock.go:140
- Defer release in saga usage: backend/application/sagas/create_node_saga.go:702

## Where It’s Used
- Ensuring a user's default graph exists (per‑user lock):
  - backend/application/sagas/create_node_saga.go:693
  - Resource name: `default_graph_creation_<userID>`

## Choosing Duration and Timeouts
- `lockDuration`: must comfortably exceed the critical section. If it expires too early, another worker can acquire the same lock while work is still in flight.
- `timeout` (for TryAcquire): how long to keep retrying before failing the operation.
- Both are contextual; start conservative and tune with metrics.

## Safety Notes and Extensions
- Idempotency checks still required inside the critical section (double‑check state after acquiring the lock).
- Fencing tokens (advanced): include `LockID` with downstream writes and reject stale holders.
- Clock skew: relies on near‑synchronized clocks (Lambda). Consider server time sources if running elsewhere.
- Extension/heartbeat: not implemented here (see `Extend()` placeholder). Add if work can exceed `lockDuration`.

## Minimal Usage Pattern
```go
lock, err := distributedLock.TryAcquireLock(ctx,
    fmt.Sprintf("graph_update_%s", graphID),
    userID,
    30*time.Second,  // lockDuration
    5*time.Second,   // timeout
)
if err != nil { return err }
defer lock.Release(ctx)

// critical section (verify state, mutate, commit)
```

## Quick Checklist
- Choose a precise `resourceName` to minimize contention.
- Pick `lockDuration` > worst‑case critical section.
- Use TryAcquire with a bounded `timeout` and backoff.
- Double‑check state after locking (idempotency).
- Log lock acquisition and release with resource, owner, and durations.

## Related Code References
- Lock implementation: backend/infrastructure/persistence/dynamodb/distributed_lock.go:15
- DI provider: backend/infrastructure/di/providers.go:260
- Saga usage: backend/application/sagas/create_node_saga.go:693

