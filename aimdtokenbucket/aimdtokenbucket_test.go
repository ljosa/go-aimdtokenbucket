package aimdtokenbucket

import (
	"log"
	"math/rand"
	"testing"
	"time"
)

func TestNewStop(t *testing.T) {
	tb := NewAIMDTokenBucket(17.0, 34, 3*time.Second)
	defer tb.Stop()
}

func TestIntial(t *testing.T) {
	tb := NewAIMDTokenBucket(1.5, 4, 3*time.Second)
	defer tb.Stop()
	n := len(tb.Bucket)
	if n != 0 {
		t.Fatal("Unexpected initial fill", n)
	}
}

func TestRate(t *testing.T) {
	tb := NewAIMDTokenBucket(1.5, 4, 3*time.Second)
	defer tb.Stop()
	time.Sleep(2 * time.Second)
	n := len(tb.Bucket)
	if n != 3 {
		t.Fatal("Unexpected fill after 2 s:", n)
	}
}

func TestBackoff(t *testing.T) {
	rand.Seed(1)
	tb := NewAIMDTokenBucket(1.5, 4, 24*time.Hour)
	defer tb.Stop()
	tb.Backoff()
	time.Sleep(3300 * time.Millisecond)
	n := len(tb.Bucket)
	if n != 3 {
		t.Fatal("Unexpected fill after backoff and 4 s:", n)
	}
}

func TestRecover(t *testing.T) {
	tb := NewAIMDTokenBucket(1.5, 4, time.Second)
	defer tb.Stop()
	tb.Backoff()
	time.Sleep(time.Second) // wait for rate to recover
	// Drain the bucket.
	for len(tb.Bucket) > 0 {
		<-tb.Bucket
	}
	time.Sleep(2 * time.Second)
	n := len(tb.Bucket)
	if n != 3 {
		t.Fatal("Unexpected fill after backoff and recovery:", n)
	}
}

func TestTwoClients(t *testing.T) {
	totalMaxRate := 50.0
	bucketSize := 10
	tb1 := NewAIMDTokenBucket(totalMaxRate, bucketSize, 6*time.Second)
	defer tb1.Stop()
	server := make(chan chan bool)
	go func() {
		starts := make([]time.Time, 5)
		for {
			starts[4] = starts[3]
			starts[3] = starts[2]
			starts[2] = starts[1]
			starts[1] = starts[0]
			starts[0] = time.Now()
			rch := <-server
			if starts[4].IsZero() {
				rch <- true
			} else {
				elapsed := time.Now().Sub(starts[4]).Seconds()
				rch <- elapsed/5 >= 1.0/totalMaxRate
			}
		}
	}()
	client := func(name string, tb *AIMDTokenBucket, rch chan float64) {
		var rate float64
		n := 50
		for i := 0; i < n; i++ {
			rate = <-tb.Bucket
			//log.Println(name, rate, "per second")
			ch := make(chan bool)
			server <- ch
			if !<-ch {
				//log.Println(name, "backoff from", rate)
				tb.Backoff()
				n = n + 1
			}
		}
		rch <- rate
	}
	ch1 := make(chan float64)
	go client("client1", tb1, ch1)
	rate1 := <-ch1
	if rate1 < 40 {
		log.Fatal("rate for single client became", rate1, "-- expected closer to", totalMaxRate)
	}
	// Start another client
	tb2 := NewAIMDTokenBucket(totalMaxRate, bucketSize, 6*time.Second)
	defer tb2.Stop()
	go client("client1", tb1, ch1)
	ch2 := make(chan float64)
	go client("client2", tb2, ch2)
	rate1 = <-ch1
	rate2 := <-ch2
	if rate1 < 20 || rate1 > 30 || rate2 < 20 || rate2 > 30 {
		log.Fatal("rate for two clients became", rate1, "and", rate2, "-- expected closer to", totalMaxRate/2.0)
	}
}
