package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm/clause"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Plan struct {
	Name     string
	ApiKey   string
	Priority int
}

type User struct {
	ID     int
	Plan   Plan
	ApiKey string
}

type SendSmsRequest struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}

type Balance struct {
	CustomerId    int   `gorm:"primary_key;column:customer_id"`
	BalanceBigint int64 `gorm:"column:balance_bigint"`
}

func (Balance) TableName() string {
	return "balances"
}

type Stats struct {
	totalRequests           atomic.Int64
	successRequests         atomic.Int64
	failedRequests          atomic.Int64
	paymentRequiredRequests atomic.Int64
	totalLatency            atomic.Int64
	minLatency              atomic.Int64
	maxLatency              atomic.Int64
}

type Simulator struct {
	serverURL    string
	users        []User
	targetRPS    int
	duration     time.Duration
	stats        Stats
	httpClient   *http.Client
	phoneNumbers []string
	messages     []string
	db           *gorm.DB
}

var plans = []Plan{
	{Name: "free", ApiKey: "da20bfca-2625-4afc-87a1-528c777eaa07", Priority: 1},
	{Name: "pro", ApiKey: "1c6f516a-dc99-4d76-8ab7-2c53b0c2da41", Priority: 2},
	{Name: "enterprise", ApiKey: "8422f6ac-a32e-4903-8edf-bc951c70ef7f", Priority: 3},
}

func NewSimulator(serverURL string, numUsers, targetRPS int, duration time.Duration, db *gorm.DB) *Simulator {
	return &Simulator{
		serverURL: serverURL,
		targetRPS: targetRPS,
		duration:  duration,
		users:     generateUsers(numUsers),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 1000,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		phoneNumbers: generatePhoneNumbers(1000),
		messages:     generateMessages(100),
		db:           db,
	}
}

func generateUsers(numUsers int) []User {
	users := make([]User, numUsers)

	// Distribution: 50% free, 30% pro, 20% enterprise
	freeCount := numUsers * 50 / 100
	proCount := numUsers * 30 / 100
	// enterpriseCount := numUsers - freeCount - proCount

	for i := 0; i < numUsers; i++ {
		var plan Plan
		if i < freeCount {
			plan = plans[0] // free
		} else if i < freeCount+proCount {
			plan = plans[1] // pro
		} else {
			plan = plans[2] // enterprise
		}

		users[i] = User{
			ID:     i + 1,
			Plan:   plan,
			ApiKey: plan.ApiKey,
		}
	}

	// Shuffle users for random distribution
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(users), func(i, j int) {
		users[i], users[j] = users[j], users[i]
	})

	return users
}

// generatePhoneNumbers creates a pool of random phone numbers
func generatePhoneNumbers(count int) []string {
	phones := make([]string, count)
	for i := 0; i < count; i++ {
		phones[i] = fmt.Sprintf("+1%010d", rand.Intn(9999999999))
	}
	return phones
}

// generateMessages creates a pool of random messages
func generateMessages(count int) []string {
	templates := []string{
		"Your verification code is %d",
		"Welcome to our service! Your code: %d",
		"OTP: %d - Valid for 5 minutes",
		"Your authentication code is %d",
		"Login code: %d",
		"Security code: %d - Do not share",
		"Confirmation code: %d",
		"Your one-time password: %d",
		"Access code: %d",
		"Verification: %d",
	}

	messages := make([]string, count)
	for i := 0; i < count; i++ {
		template := templates[rand.Intn(len(templates))]
		messages[i] = fmt.Sprintf(template, rand.Intn(999999))
	}
	return messages
}

// sendRequest sends a single SMS request
func (s *Simulator) sendRequest(ctx context.Context, user User) error {
	// Create random request body
	reqBody := SendSmsRequest{
		PhoneNumber: s.phoneNumbers[rand.Intn(len(s.phoneNumbers))],
		Message:     s.messages[rand.Intn(len(s.messages))],
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", s.serverURL+"/v1/sms/send", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("create request error: %w", err)
	}

	// Set headers (matching your middleware requirements)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", user.ApiKey)
	req.Header.Set("X-Auth-User-Id", fmt.Sprintf("%d", user.ID))

	// Send request and measure latency
	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()

	// Update latency stats
	s.stats.totalLatency.Add(latency)

	// Update min latency
	for {
		currentMin := s.stats.minLatency.Load()
		if currentMin == 0 || latency < currentMin {
			if s.stats.minLatency.CompareAndSwap(currentMin, latency) {
				break
			}
		} else {
			break
		}
	}

	// Update max latency
	for {
		currentMax := s.stats.maxLatency.Load()
		if latency > currentMax {
			if s.stats.maxLatency.CompareAndSwap(currentMax, latency) {
				break
			}
		} else {
			break
		}
	}

	if err != nil {
		s.stats.failedRequests.Add(1)
		return fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusPaymentRequired {
		s.stats.paymentRequiredRequests.Add(1)
		return nil
	}

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		s.stats.successRequests.Add(1)
		return nil
	}

	s.stats.failedRequests.Add(1)
	return fmt.Errorf("status code: %d", resp.StatusCode)
}

// chargeUsers charges all users with random balances (multiplied by 10)
func (s *Simulator) chargeUsers(ctx context.Context) error {
	fmt.Printf("💰 Charging users with random balances...\n")

	// Generate random balances for each user
	balances := make([]Balance, len(s.users))
	totalBalance := int64(0)

	for i, user := range s.users {
		// Generate random base value between 1 and 1000
		baseValue := rand.Intn(1000) + 1
		// Multiply by 10 to get values like 10, 50, 100, 1000, etc.
		balanceValue := int64(baseValue * 10)

		balances[i] = Balance{
			CustomerId:    user.ID,
			BalanceBigint: balanceValue,
		}
		totalBalance += balanceValue
	}

	// Insert balances in batches
	batchSize := 1000
	for i := 0; i < len(balances); i += batchSize {
		end := i + batchSize
		if end > len(balances) {
			end = len(balances)
		}

		batch := balances[i:end]

		// Insert or replace existing records
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "customer_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"balance_bigint"}),
		}).Create(&batch).Error; err != nil {
			return fmt.Errorf("failed to insert balance batch: %w", err)
		}
	}

	avgBalance := totalBalance / int64(len(s.users))
	fmt.Printf("✅ Charged %d users with balances (avg: %d, total: %d)\n",
		len(s.users), avgBalance, totalBalance)
	fmt.Printf("\n")

	return nil
}

// Run starts the load test
func (s *Simulator) Run(ctx context.Context) {
	fmt.Printf("🚀 Starting Load Test Simulator\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Target Server:     %s\n", s.serverURL)
	fmt.Printf("Total Users:       %d\n", len(s.users))
	fmt.Printf("Target RPS:        %d requests/second\n", s.targetRPS)
	fmt.Printf("Duration:          %s\n", s.duration)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Print user distribution
	planCounts := make(map[string]int)
	for _, user := range s.users {
		planCounts[user.Plan.Name]++
	}
	fmt.Printf("User Distribution:\n")
	for _, plan := range plans {
		count := planCounts[plan.Name]
		percentage := float64(count) / float64(len(s.users)) * 100
		fmt.Printf("  %s: %d users (%.1f%%)\n", plan.Name, count, percentage)
	}
	fmt.Printf("\n")

	// Initialize min latency to max value
	s.stats.minLatency.Store(int64(^uint64(0) >> 1))

	// Create ticker for rate limiting
	requestInterval := time.Second / time.Duration(s.targetRPS)
	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, s.duration)
	defer cancel()

	// Start statistics printer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.printStats(testCtx)
	}()

	// Start load test
	fmt.Printf("🔥 Load test started at %s\n\n", time.Now().Format("15:04:05"))
	startTime := time.Now()

	var requestWg sync.WaitGroup
	requestCount := 0

	for {
		select {
		case <-testCtx.Done():
			// Wait for all requests to complete
			requestWg.Wait()

			// Wait for stats printer to finish
			wg.Wait()

			// Print final report
			s.printFinalReport(time.Since(startTime))
			return

		case <-ticker.C:
			// Send one request per tick
			user := s.users[requestCount%len(s.users)]
			requestCount++
			s.stats.totalRequests.Add(1)

			requestWg.Add(1)
			go func(u User) {
				defer requestWg.Done()
				_ = s.sendRequest(testCtx, u)
			}(user)
		}
	}
}

// printStats prints real-time statistics
func (s *Simulator) printStats(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastTotal := int64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			total := s.stats.totalRequests.Load()
			success := s.stats.successRequests.Load()
			failed := s.stats.failedRequests.Load()
			noBalance := s.stats.paymentRequiredRequests.Load()

			// Calculate current RPS
			currentTotal := total
			currentTime := time.Now()
			elapsed := currentTime.Sub(lastTime).Seconds()
			currentRPS := float64(currentTotal-lastTotal) / elapsed
			lastTotal = currentTotal
			lastTime = currentTime

			// Calculate average latency
			avgLatency := int64(0)
			if total > 0 {
				avgLatency = s.stats.totalLatency.Load() / total
			}

			minLatency := s.stats.minLatency.Load()
			if minLatency == int64(^uint64(0)>>1) {
				minLatency = 0
			}
			maxLatency := s.stats.maxLatency.Load()

			// Calculate success rate
			successRate := float64(0)
			if total > 0 {
				successRate = ((float64(success) + float64(noBalance)) / float64(total)) * 100
			}

			fmt.Printf("📊 Total: %6d | Success: %6d | No-Balance: %6d | Failed: %6d | RPS: %7.1f | Success Rate: %5.1f%% | Latency (ms): avg=%d min=%d max=%d\n",
				total, success, noBalance, failed, currentRPS, successRate, avgLatency, minLatency, maxLatency)
		}
	}
}

// printFinalReport prints the final test report
func (s *Simulator) printFinalReport(duration time.Duration) {
	total := s.stats.totalRequests.Load()
	success := s.stats.successRequests.Load()
	failed := s.stats.failedRequests.Load()

	avgLatency := int64(0)
	if total > 0 {
		avgLatency = s.stats.totalLatency.Load() / total
	}

	minLatency := s.stats.minLatency.Load()
	if minLatency == int64(^uint64(0)>>1) {
		minLatency = 0
	}
	maxLatency := s.stats.maxLatency.Load()

	successRate := float64(0)
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}

	actualRPS := float64(total) / duration.Seconds()

	fmt.Printf("\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📈 FINAL LOAD TEST REPORT\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Test Duration:       %s\n", duration.Round(time.Second))
	fmt.Printf("Total Requests:      %d\n", total)
	fmt.Printf("Successful:          %d (%.2f%%)\n", success, successRate)
	fmt.Printf("Failed:              %d (%.2f%%)\n", failed, 100-successRate)
	fmt.Printf("Average RPS:         %.2f requests/sec\n", actualRPS)
	fmt.Printf("Target RPS:          %d requests/sec\n", s.targetRPS)
	fmt.Printf("\nLatency Statistics:\n")
	fmt.Printf("  Average:           %d ms\n", avgLatency)
	fmt.Printf("  Minimum:           %d ms\n", minLatency)
	fmt.Printf("  Maximum:           %d ms\n", maxLatency)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if successRate >= 99.0 {
		fmt.Printf("✅ Test PASSED - Success rate: %.2f%%\n", successRate)
	} else if successRate >= 95.0 {
		fmt.Printf("⚠️  Test WARNING - Success rate: %.2f%%\n", successRate)
	} else {
		fmt.Printf("❌ Test FAILED - Success rate: %.2f%%\n", successRate)
	}
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

func main() {
	ctx := context.Background()

	// Check command-line arguments
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s [charge|run]\n", os.Args[0])
		fmt.Printf("  charge - Charge users with random balances\n")
		fmt.Printf("  run    - Run load test (will also charge users first)\n")
		os.Exit(1)
	}

	command := os.Args[1]
	if command != "charge" && command != "run" {
		fmt.Printf("❌ Invalid command: %s\n", command)
		fmt.Printf("Usage: %s [charge|run]\n", os.Args[0])
		os.Exit(1)
	}

	// Configuration
	const (
		serverURL    = "http://server:8080" // Your server URL
		numUsers     = 100000               // 100k users
		targetRPS    = 2000                 // 2k requests per second
		testDuration = 5 * time.Minute      // Test duration (adjust as needed)
	)

	// Database configuration (adjust as needed)
	dsn := "host=postgres user=messenger password=messenger dbname=messenger port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("❌ Failed to connect to database: %v\n", err)
		return
	}

	// Get underlying SQL DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Printf("❌ Failed to get SQL DB: %v\n", err)
		return
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Create simulator
	s := NewSimulator(serverURL, numUsers, targetRPS, testDuration, db)

	// Execute based on command
	switch command {
	case "charge":
		if err := s.chargeUsers(ctx); err != nil {
			fmt.Printf("❌ Failed to charge users: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Successfully charged users\n")

	case "run":
		// Run the test
		s.Run(ctx)
	}
}
