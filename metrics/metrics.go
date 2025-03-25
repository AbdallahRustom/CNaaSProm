package metrics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsRegistry = prometheus.NewRegistry()
)

// Fetch JSON data from a single URL
func fetchJSONData(apiURL string) (map[string]map[string]int, error) {
	log.Printf("Fetching data from URL: %s", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JSON data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var stats map[string]map[string]int
	err = json.Unmarshal(data, &stats)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return stats, nil
}

// Combine JSON data from multiple URLs
func fetchAndCombineJSONData(MetricsCategories []string, queryParams string, StatisticServerAddr string, StatisticServerPort uint) (map[string]map[string]int, error) {
	combinedData := make(map[string]map[string]int)
	baseURL := fmt.Sprintf("http://%s:%d/nnfcm-statistics/v2/stats", StatisticServerAddr, StatisticServerPort)

	for _, MetricsCategory := range MetricsCategories {

		fullURL := fmt.Sprintf("%s/%s?operatorIdentifier=%s", baseURL, MetricsCategory, queryParams)

		data, err := fetchJSONData(fullURL)
		if err != nil {
			log.Printf("Error fetching data from %s: %v", fullURL, err)
			continue
		}

		for category, metrics := range data {
			prefixedCategory := fmt.Sprintf("%s_%s", MetricsCategory, category)
			if _, exists := combinedData[prefixedCategory]; !exists {
				combinedData[prefixedCategory] = make(map[string]int)
			}
			for metricName, value := range metrics {
				combinedData[prefixedCategory][metricName] += value
			}
		}
	}

	return combinedData, nil
}

func registerMetricsFromJSON(data map[string]map[string]int) error {
	for category, metrics := range data {
		for metricName, value := range metrics {
			metric := prometheus.NewGauge(prometheus.GaugeOpts{
				Name: fmt.Sprintf("%s_%s", category, metricName),
				Help: fmt.Sprintf("Metric %s from category %s", metricName, category),
			})

			metric.Set(float64(value))

			err := metricsRegistry.Register(metric)
			if err != nil {
				if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
					existingMetric := are.ExistingCollector.(prometheus.Gauge)
					existingMetric.Set(float64(value))
				} else {
					log.Printf("Error registering metric %s: %v", metricName, err)
				}
			}
		}
	}
	return nil
}

// HTTP handler for Prometheus metrics
func MetricsHandler(MetricsCategories []string, queryParams string, StatisticServerAddr string, StatisticServerPort uint) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fetch and combine JSON data from all URLs
		combinedData, err := fetchAndCombineJSONData(MetricsCategories, queryParams, StatisticServerAddr, StatisticServerPort)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch and combine JSON data: %v", err), http.StatusInternalServerError)
			return
		}

		// Clear the registry and re-register metrics
		metricsRegistry = prometheus.NewRegistry()
		err = registerMetricsFromJSON(combinedData)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to register metrics: %v", err), http.StatusInternalServerError)
			return
		}

		// Serve metrics
		promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
}
