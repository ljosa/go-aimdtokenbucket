// Package aimdtokenbucket implements the additive
// increase/multiplicative decrease (AIMD) feedback control algorithm.
package aimdtokenbucket

import (
	"math"
	//	"math/rand"
	"time"
)

type Void struct{}

var void = Void{}

type AIMDTokenBucket struct {
	// Consume tokens by receiving from the `Bucket` channel.
	Bucket          <-chan float64
	backoff         chan<- Void
	ticker          *time.Ticker
	backoffFunction func(float64) float64
}

// NewAIMDTokenBucket returns a new AIMD token bucket. The bucket will
// be full initially. You can consume tokens by reading from the
// Bucket channel. The bucket will refill gradually at a rate that is
// initially `maxFlowPerSecond` tokens per second and will never
// exceed that rate. The refill rate will be reduced by up to 50%
// every time a backoff signal is received. The refill rate will
// recover gradually and linearly to its initial value over a period
// of up to `recoveryDuration`.
//
// The value of a token is the rate per second at the time the token
// was added to the bucket. (The value is useful as a metric, to
// understand how the system is behaving.)
func NewAIMDTokenBucket(maxFlowPerSecond float64, bucketSize int, recoveryDuration time.Duration) *AIMDTokenBucket {
	d := 10 * time.Millisecond // how often water is added to the bucket
	var tb AIMDTokenBucket
	tb.ticker = time.NewTicker(d)
	maxFlowPerD := maxFlowPerSecond * float64(d) / float64(time.Second)
	recoveryPerD := maxFlowPerD * float64(d) / float64(recoveryDuration)
	flowPerD := maxFlowPerD
	backoffChannel := make(chan Void)
	tb.backoff = backoffChannel
	// Start with an empty bucket. Maintain the fill as a float64
	// because less than one token may be added each `d`.
	fill := 0.0
	bucket := make(chan float64, bucketSize)
	tb.Bucket = bucket
	go func() {
		for {
			select {
			case <-tb.ticker.C:
				flowPerD = math.Min(flowPerD+recoveryPerD, maxFlowPerD)
				flowPerSecond := flowPerD * float64(time.Second) / float64(d)
				fill = fill + flowPerD
				for fill >= 1 {
					sendToken(bucket, flowPerSecond)
					fill = fill - 1
				}
			case <-backoffChannel:
				flowPerD = flowPerD / 2.0 //(2.0 - rand.Float64())
			}
		}
	}()
	return &tb
}

// Backoff delivers a backoff signal to an AIMD token bucket. This
// reduces the rate at which tokens are added to the bucket by a
// factor of up to two.
func (tb *AIMDTokenBucket) Backoff() {
	tb.backoff <- void
}

// Stop turns off an AIMD token bucket.  After Stop, no more tokens
// will be added to the bucket.  Stop does not close the bucket
// channel in order to prevent a read from the channel from succeeding
// incorrectly.
func (tb *AIMDTokenBucket) Stop() {
	tb.ticker.Stop()
}

func sendToken(bucket chan float64, ratePerSecond float64) {
	select {
	case bucket <- ratePerSecond:
	default:
	}
}
