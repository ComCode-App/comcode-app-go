package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var eventTypes = []string{
	"card.view",
	"card.qr.scan",
	"contact.phone.click",
	"contact.email.click",
	"contact.website.click",
	"vcard.download",
	"social.instagram.click",
}

var sampleUserAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.122 Mobile Safari/537.36",
	"Mozilla/5.0 (iPad; CPU OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0",
}

var sampleReferers = []string{
	"https://google.com/search?q=business+cards",
	"https://instagram.com/p/example",
	"https://linkedin.com/in/card-owner",
	"https://t.co/shortlink",
	"",
}

func main() {
	url := flag.String("url", "http://localhost:8080/v1/events", "Target event ingestion API URL")
	total := flag.Int("total", 1000, "Total number of events to ingest")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent worker goroutines")
	flag.Parse()

	fmt.Printf("Starting Analytics Ingestion Benchmark\nTarget: %s\nTotal Events: %d\nConcurrency: %d\n\n", *url, *total, *concurrency)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	var succeeded uint64
	var failed uint64
	var totalDurationNs uint64

	start := time.Now()
	var wg sync.WaitGroup
	eventCh := make(chan int, *total)

	for i := 0; i < *total; i++ {
		eventCh <- i
	}
	close(eventCh)

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for idx := range eventCh {
				cardID := fmt.Sprintf("card_%04d", idx%100)
				eventType := eventTypes[rand.Intn(len(eventTypes))]
				ua := sampleUserAgents[rand.Intn(len(sampleUserAgents))]
				ref := sampleReferers[rand.Intn(len(sampleReferers))]
				fakeIP := fmt.Sprintf("192.168.%d.%d", rand.Intn(250)+1, rand.Intn(250)+1)

				body := map[string]any{
					"type": eventType,
					"subject": map[string]string{
						"type": "card",
						"id":   cardID,
					},
					"session_id": fmt.Sprintf("sess_%d_%d", workerID, idx),
					"properties": map[string]any{
						"campaign": "bench-run",
						"source":   "qr",
					},
				}

				payload, _ := json.Marshal(body)
				req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, *url, bytes.NewReader(payload))
				if err != nil {
					atomic.AddUint64(&failed, 1)
					continue
				}

				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", ua)
				req.Header.Set("Referer", ref)
				req.Header.Set("X-Forwarded-For", fakeIP)

				t0 := time.Now()
				resp, err := client.Do(req)
				dur := time.Since(t0)
				atomic.AddUint64(&totalDurationNs, uint64(dur.Nanoseconds()))

				if err != nil || (resp != nil && resp.StatusCode >= 300) {
					atomic.AddUint64(&failed, 1)
				} else {
					atomic.AddUint64(&succeeded, 1)
				}

				if resp != nil {
					_ = resp.Body.Close()
				}
			}
		}(i)
	}

	wg.Wait()
	totalElapsed := time.Since(start)

	succ := atomic.LoadUint64(&succeeded)
	fail := atomic.LoadUint64(&failed)
	totDur := atomic.LoadUint64(&totalDurationNs)
	avgLatency := time.Duration(0)
	if succ+fail > 0 {
		avgLatency = time.Duration(totDur / (succ + fail))
	}
	throughput := float64(succ+fail) / totalElapsed.Seconds()

	fmt.Println("================ Benchmark Results ================")
	fmt.Printf("Total Requests:     %d\n", succ+fail)
	fmt.Printf("Successful (202):   %d\n", succ)
	fmt.Printf("Failed:             %d\n", fail)
	fmt.Printf("Total Elapsed Time: %v\n", totalElapsed)
	fmt.Printf("Average Latency:    %v\n", avgLatency)
	fmt.Printf("Throughput:         %.2f req/sec\n", throughput)
	fmt.Println("===================================================")
}
