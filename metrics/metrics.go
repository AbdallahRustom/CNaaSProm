package metrics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsRegistry = prometheus.NewRegistry()
)

// Fetch JSON data from a single URL
func fetchJSONData(apiURL string, dataType string) (interface{}, error) {
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

	switch dataType {
	case "statistics":
		var stats map[string]map[string]int
		err = json.Unmarshal(data, &stats)
		if err != nil {
			return nil, fmt.Errorf("failed to parse statistics JSON: %v", err)
		}
		return stats, nil

	case "monitoring":
		var monitor map[string]string
		err = json.Unmarshal(data, &monitor)
		if err != nil {
			return nil, fmt.Errorf("failed to parse monitoring JSON: %v", err)
		}
		return monitor, nil

	default:
		return nil, fmt.Errorf("unknown data type: %s", dataType)
	}
}

// Combine JSON data from multiple URLs
func fetchAndCombineJSONData(
	MetricsCategories []string,
	queryParams string,
	serverAddr string,
	serverPort uint,
	dataType string, // "statistics" or "monitoring"
) (map[string]map[string]int, error) {
	combinedData := make(map[string]map[string]int)

	// Determine the base URL based on the data type
	var baseURL string
	if dataType == "statistics" {
		baseURL = fmt.Sprintf("http://%s:%d/nnfcm-statistics/v2/stats", serverAddr, serverPort)
	} else if dataType == "monitoring" {
		baseURL = fmt.Sprintf("http://%s:%d/nnfcm-monitoring/v2", serverAddr, serverPort)
	} else {
		return nil, fmt.Errorf("invalid data type: %s", dataType)
	}

	// Fetch and combine data for each category
	for _, MetricsCategory := range MetricsCategories {
		fullURL := fmt.Sprintf("%s/%s?operatorIdentifier=%s", baseURL, MetricsCategory, queryParams)

		// Fetch data
		data, err := fetchJSONData(fullURL, dataType)
		if err != nil {
			log.Printf("Error fetching data from %s: %v", fullURL, err)
			continue
		}

		// Handle data based on its type
		switch dataType {
		case "statistics":
			statsData, ok := data.(map[string]map[string]int)
			if !ok {
				return nil, fmt.Errorf("unexpected data type for statistics")
			}

			for category, metrics := range statsData {
				prefixedCategory := fmt.Sprintf("%s_%s", MetricsCategory, category)
				if _, exists := combinedData[prefixedCategory]; !exists {
					combinedData[prefixedCategory] = make(map[string]int)
				}
				for metricName, value := range metrics {
					combinedData[prefixedCategory][metricName] += value
				}
			}

		case "monitoring":
			monitorData, ok := data.(map[string]string)
			if !ok {
				return nil, fmt.Errorf("unexpected data type for monitoring")
			}

			// Monitoring data doesn't have nested categories, so we use the MetricsCategory as the prefix
			prefixedCategory := MetricsCategory
			if _, exists := combinedData[prefixedCategory]; !exists {
				combinedData[prefixedCategory] = make(map[string]int)
			}

			for metricName, value := range monitorData {
				// Remove non-numeric characters (e.g., "bps")
				cleanedValue := strings.Fields(value)[0] // Extract the numeric part before any spaces or units

				// Convert the cleaned string to a float
				floatValue, err := strconv.ParseFloat(cleanedValue, 64)
				if err != nil {
					log.Printf("Error converting value '%s' for metric '%s' to float: %v", value, metricName, err)
					continue
				}

				// Convert float to int (if needed) and add to combinedData
				combinedData[prefixedCategory][metricName] += int(floatValue)
			}

		default:
			return nil, fmt.Errorf("invalid data type: %s", dataType)
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
func MetricsHandler(
	MetricsStatisticsCategories []string,
	MetricsMonitoringCategories []string,
	queryParams string,
	StatisticServerAddr string,
	StatisticServerPort uint,
	MonitoringServerAddr string,
	MonitoringServerPort uint,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		combinedData := make(map[string]map[string]int)
		errorChan := make(chan error, 2)

		// Fetch statistics data if configuration is provided
		if len(MetricsStatisticsCategories) > 0 && StatisticServerAddr != "" && StatisticServerPort != 0 {
			statsChan := make(chan map[string]map[string]int)
			go func() {
				data, err := fetchAndCombineJSONData(MetricsStatisticsCategories, queryParams, StatisticServerAddr, StatisticServerPort, "statistics")
				if err != nil {
					errorChan <- fmt.Errorf("failed to fetch statistics data: %v", err)
					return
				}
				statsChan <- data
			}()

			select {
			case statsData := <-statsChan:
				for category, metrics := range statsData {
					combinedData[category] = metrics
				}
			case err := <-errorChan:
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Fetch monitoring data if configuration is provided
		if len(MetricsMonitoringCategories) > 0 && MonitoringServerAddr != "" && MonitoringServerPort != 0 {
			monitorChan := make(chan map[string]map[string]int)
			go func() {
				data, err := fetchAndCombineJSONData(MetricsMonitoringCategories, queryParams, MonitoringServerAddr, MonitoringServerPort, "monitoring")
				if err != nil {
					errorChan <- fmt.Errorf("failed to fetch monitoring data: %v", err)
					return
				}
				monitorChan <- data
			}()

			select {
			case monitorData := <-monitorChan:
				for category, metrics := range monitorData {
					combinedData[category] = metrics
				}
			case err := <-errorChan:
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// If no valid configuration is provided, return an error
		if len(combinedData) == 0 {
			http.Error(w, "No valid configuration provided for statistics or monitoring", http.StatusBadRequest)
			return
		}

		// Clear the registry and re-register metrics
		metricsRegistry = prometheus.NewRegistry()
		err := registerMetricsFromJSON(combinedData)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to register metrics: %v", err), http.StatusInternalServerError)
			return
		}

		// Serve metrics
		promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
}
