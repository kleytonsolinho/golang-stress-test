package stresstest

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type MapStatusRequests map[int]int

type StressReport struct {
	Requests            int
	Failed              int
	Succeeded           int
	TimedOut            int
	TotalTime           float64
	AverageTime         float64
	FastestTime         int64
	SlowestTime         int64
	PercentageSucceeded float64
	PercentageFailed    float64
	PercentageTimedOut  float64
	StatusRequests      MapStatusRequests
}

func NewStressReport() *StressReport {
	return &StressReport{
		Requests:            0,
		Failed:              0,
		Succeeded:           0,
		TimedOut:            0,
		TotalTime:           0,
		AverageTime:         0,
		FastestTime:         0,
		SlowestTime:         0,
		PercentageSucceeded: 0,
		PercentageFailed:    0,
		PercentageTimedOut:  0,
		StatusRequests:      make(MapStatusRequests),
	}
}

type IStress interface {
	Run() error
	PrintReport()
}

type Stress struct {
	URL         string
	Method      string
	Concurrency int
	Requests    int
	Timeout     int
	Verbose     bool
	Report      *StressReport
	VerifyTls   bool
	mu          sync.Mutex
}

func NewStress(url string, method string, concurrency int, requests int, timeout int, verifyTls bool, verbose bool) *Stress {
	report := NewStressReport()
	return &Stress{
		URL:         url,
		Method:      method,
		Concurrency: concurrency,
		Requests:    requests,
		Timeout:     timeout,
		Verbose:     verbose,
		Report:      report,
		VerifyTls:   verifyTls,
		mu:          sync.Mutex{},
	}
}

func (s *Stress) Run() error {
	fmt.Println("Running stress test...")
	s.run()
	return nil
}

func (s *Stress) PrintReport() {
	fmt.Println("--- Report ---")
	fmt.Println("Requests:", s.Report.Requests)
	fmt.Println("Failed:", s.Report.Failed)
	fmt.Println("Succeeded:", s.Report.Succeeded)
	fmt.Println("TimedOut:", s.Report.TimedOut)
	fmt.Println("TotalTime:", s.Report.TotalTime, "ms")
	fmt.Println("AverageTime:", s.Report.AverageTime, "ms")
	fmt.Println("FastestTime:", s.Report.FastestTime, "ms")
	fmt.Println("SlowestTime:", s.Report.SlowestTime, "ms")
	fmt.Println("PercentageSucceeded:", s.Report.PercentageSucceeded, "%")
	fmt.Println("PercentageFailed:", s.Report.PercentageFailed, "%")
	fmt.Println("PercentageTimedOut:", s.Report.PercentageTimedOut, "%")
	fmt.Println("--- Requests per status code ---")
	for status, requests := range s.Report.StatusRequests {
		fmt.Println("Status", fmt.Sprint(status)+":", requests, "requests")
	}
}

func (s *Stress) run() {
	start := time.Now()

	var wg sync.WaitGroup

	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		i := i

		go func() {
			defer wg.Done()
			for j := 0; j < s.Requests/s.Concurrency; j++ {
				s.runRequest(i + 1)
			}
		}()
	}

	for i := 0; i < s.Requests%s.Concurrency; i++ {
		wg.Add(1)
		i := i

		go func() {
			defer wg.Done()
			s.runRequest(i + 1)
		}()
	}

	wg.Wait()
	elapsed := time.Since(start).Milliseconds()

	s.Report.TotalTime = float64(elapsed)
	s.Report.AverageTime = s.Report.TotalTime / float64(s.Report.Requests)
	s.Report.PercentageSucceeded = float64(s.Report.Succeeded) / float64(s.Report.Requests) * 100
	s.Report.PercentageFailed = float64(s.Report.Failed) / float64(s.Report.Requests) * 100
	s.Report.PercentageTimedOut = float64(s.Report.TimedOut) / float64(s.Report.Requests) * 100
	fmt.Println("Finished stress test")
}

func (s *Stress) runRequest(concurrencyGroup int) {
	start := time.Now()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !s.VerifyTls},
	}

	client := &http.Client{
		Timeout:   time.Duration(s.Timeout) * time.Second,
		Transport: tr,
	}

	req, err := http.NewRequest(s.Method, s.URL, nil)
	if err != nil {
		panic(err)
	}

	res, err := client.Do(req)

	elapsed := time.Since(start).Milliseconds()

	if s.Verbose {
		fmt.Print(fmt.Sprint(concurrencyGroup) + " | " + fmt.Sprint(s.Report.Requests+1) + " " + s.Method + " " + s.URL)
		fmt.Println(" Time:", elapsed, "ms, Status:", res.StatusCode)
	}

	s.updateReport(res, err, elapsed)
}

func (s *Stress) updateReport(res *http.Response, err error, elapsed int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		fmt.Println(err)
		if strings.Contains(err.Error(), "connection refused") {
			panic(err)
		}
		if err.Error() == http.ErrHandlerTimeout.Error() {
			s.Report.TimedOut++
		}
		s.Report.Failed++
	} else {
		if res.StatusCode != 200 {
			s.Report.Failed++
		} else {
			s.Report.Succeeded++
		}
		if _, ok := s.Report.StatusRequests[res.StatusCode]; !ok {
			s.Report.StatusRequests[res.StatusCode] = 0
		}
		s.Report.StatusRequests[res.StatusCode]++
	}

	s.Report.Requests++

	if elapsed < s.Report.FastestTime || s.Report.FastestTime == 0 {
		s.Report.FastestTime = elapsed
	}

	if elapsed > s.Report.SlowestTime {
		s.Report.SlowestTime = elapsed
	}
}
