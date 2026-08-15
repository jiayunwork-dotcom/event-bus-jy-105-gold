package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenBucket struct {
	Rate       float64
	Capacity   float64
	Tokens     float64
	LastRefill time.Time
	mu         sync.Mutex
}

type BackpressureController struct {
	mu      sync.RWMutex
	buckets map[string]*TokenBucket
	redis   *redis.Client
	alertCh chan AlertEvent
}

type AlertEvent struct {
	SubscriptionID string
	TenantID       string
	AlertType      string
	Message        string
}

func NewBackpressureController(rdb *redis.Client) *BackpressureController {
	return &BackpressureController{
		buckets: make(map[string]*TokenBucket),
		redis:   rdb,
		alertCh: make(chan AlertEvent, 100),
	}
}

func (b *BackpressureController) InitBucket(subscriptionID string, rate, burst float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.buckets[subscriptionID]; !exists {
		b.buckets[subscriptionID] = &TokenBucket{
			Rate:       rate,
			Capacity:   burst,
			Tokens:     burst,
			LastRefill: time.Now(),
		}
	}
}

func (b *BackpressureController) UpdateRate(subscriptionID string, rate, burst float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if bucket, exists := b.buckets[subscriptionID]; exists {
		bucket.Rate = rate
		bucket.Capacity = burst
		if bucket.Tokens > burst {
			bucket.Tokens = burst
		}
	} else {
		b.buckets[subscriptionID] = &TokenBucket{
			Rate:       rate,
			Capacity:   burst,
			Tokens:     burst,
			LastRefill: time.Now(),
		}
	}

	ctx := context.Background()
	b.redis.HSet(ctx, fmt.Sprintf("backpressure:%s", subscriptionID), map[string]interface{}{
		"rate":  rate,
		"burst": burst,
	})
}

func (b *BackpressureController) Allow(subscriptionID string) bool {
	b.mu.RLock()
	bucket, exists := b.buckets[subscriptionID]
	b.mu.RUnlock()

	if !exists {
		return true
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	b.refill(bucket)

	if bucket.Tokens >= 1 {
		bucket.Tokens--
		return true
	}

	return false
}

func (b *BackpressureController) Wait(subscriptionID string) {
	for {
		if b.Allow(subscriptionID) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (b *BackpressureController) refill(bucket *TokenBucket) {
	now := time.Now()
	elapsed := now.Sub(bucket.LastRefill).Seconds()
	bucket.Tokens = min(bucket.Tokens+elapsed*bucket.Rate, bucket.Capacity)
	bucket.LastRefill = now
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (b *BackpressureController) CheckBacklog(subscriptionID string, threshold int) bool {
	ctx := context.Background()
	key := fmt.Sprintf("backlog:%s", subscriptionID)
	count, err := b.redis.Get(ctx, key).Int()
	if err != nil {
		count = 0
	}
	return count > threshold
}

func (b *BackpressureController) UpdateBacklog(subscriptionID string, count int) {
	ctx := context.Background()
	key := fmt.Sprintf("backlog:%s", subscriptionID)
	b.redis.Set(ctx, key, count, 5*time.Minute)

	if count > 1000 {
		select {
		case b.alertCh <- AlertEvent{
			SubscriptionID: subscriptionID,
			AlertType:      "backlog_threshold",
			Message:        fmt.Sprintf("Backlog exceeds threshold: %d events", count),
		}:
		default:
			log.Printf("alert channel full, dropping alert for %s", subscriptionID)
		}
	}
}

func (b *BackpressureController) AlertChannel() <-chan AlertEvent {
	return b.alertCh
}

func (b *BackpressureController) GetBucketStats(subscriptionID string) map[string]interface{} {
	b.mu.RLock()
	bucket, exists := b.buckets[subscriptionID]
	b.mu.RUnlock()

	if !exists {
		return map[string]interface{}{
			"subscription_id": subscriptionID,
			"rate":            0,
			"capacity":        0,
			"tokens":          0,
		}
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	return map[string]interface{}{
		"subscription_id": subscriptionID,
		"rate":            bucket.Rate,
		"capacity":        bucket.Capacity,
		"tokens":          bucket.Tokens,
	}
}

func (b *BackpressureController) LoadFromRedis() {
	ctx := context.Background()
	b.mu.Lock()
	defer b.mu.Unlock()

	var cursor uint64
	for {
		keys, nextCursor, err := b.redis.Scan(ctx, cursor, "backpressure:*", 100).Result()
		if err != nil {
			break
		}

		for _, key := range keys {
			subID := key[len("backpressure:"):]
			data, err := b.redis.HGetAll(ctx, key).Result()
			if err != nil {
				continue
			}

			var rate, burst float64
			fmt.Sscanf(data["rate"], "%f", &rate)
			fmt.Sscanf(data["burst"], "%f", &burst)

			if rate > 0 {
				b.buckets[subID] = &TokenBucket{
					Rate:       rate,
					Capacity:   burst,
					Tokens:     burst,
					LastRefill: time.Now(),
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}
