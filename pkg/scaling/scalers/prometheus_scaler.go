package scalers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	httpClientTimeout = 5 * time.Second
	uptimeQuery       = "min_over_time((max(up{container=\"prometheus\"}) or vector(0))[%ds:])"
)

type prometheusScaler struct {
	httpClient     *http.Client
	metadata       *prometheusMetadata
	cooldownPeriod time.Duration
}

type prometheusMetadata struct {
	ServerAddress string  `json:"serverAddress"`
	Query         string  `json:"query"`
	Threshold     float64 `json:"threshold,string"`
}

var promQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func NewPrometheusScaler(metadata json.RawMessage, cooldownPeriod time.Duration) (Scaler, error) {
	parsedMetadata, err := parsePrometheusMetadata(metadata)
	if err != nil {
		return nil, fmt.Errorf("error creating prometheus scaler: %w", err)
	}

	client := &http.Client{
		Timeout: httpClientTimeout,
	}

	return &prometheusScaler{
		metadata:       parsedMetadata,
		httpClient:     client,
		cooldownPeriod: cooldownPeriod,
	}, nil
}

func parsePrometheusMetadata(jsonMetadata json.RawMessage) (*prometheusMetadata, error) {
	metadata := &prometheusMetadata{}
	err := json.Unmarshal(jsonMetadata, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}
	return metadata, nil
}

// golang issue: https://github.com/golang/go/issues/4013
func queryEscape(query string) string {
	queryEscaped := url.QueryEscape(query)
	plusEscaped := strings.ReplaceAll(queryEscaped, "+", "%20")

	return plusEscaped
}

func (s *prometheusScaler) executePromQuery(ctx context.Context, query string) (float64, error) {
	t := time.Now().UTC().Format(time.RFC3339)
	queryEscaped := queryEscape(query)
	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s&time=%s", s.metadata.ServerAddress, queryEscaped, t)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return -1, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return -1, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&promQueryResponse); err != nil {
		return -1, fmt.Errorf("failed to decode Prometheus response: %w", err)
	}

	var v float64 = -1

	if len(promQueryResponse.Data.Result) == 0 {
		return -1, fmt.Errorf("prometheus query %s, result is empty, prometheus metrics 'prometheus' target may be lost", query)
	} else if len(promQueryResponse.Data.Result) > 1 {
		return -1, fmt.Errorf("prometheus query %s returned multiple elements", query)
	}

	valueLen := len(promQueryResponse.Data.Result[0].Value)
	if valueLen == 0 {
		return -1, fmt.Errorf("prometheus query %s, value list in result is empty, prometheus metrics 'prometheus' target may be lost", s.metadata.Query)
	} else if valueLen < 2 {
		return -1, fmt.Errorf("prometheus query %s didn't return enough values", s.metadata.Query)
	}

	val := promQueryResponse.Data.Result[0].Value[1]
	if val != nil {
		str := val.(string)
		v, err = strconv.ParseFloat(str, 64)
		if err != nil {
			return -1, fmt.Errorf("failed to parse metric value: %w", err)
		}
	}

	if math.IsInf(v, 0) {
		return -1, fmt.Errorf("prometheus query returns %f", v)
	}

	return v, nil
}

func (s *prometheusScaler) ShouldScaleToZero(ctx context.Context) (bool, error) {
	metricValue, err := s.executePromQuery(ctx, s.metadata.Query)
	if err != nil {
		return false, fmt.Errorf("failed to execute prometheus query %s: %w", s.metadata.Query, err)
	}

	if metricValue == -1 {
		return false, nil
	}
	if metricValue < s.metadata.Threshold {
		return true, nil
	}
	return false, nil
}

func (s *prometheusScaler) ShouldScaleFromZero(ctx context.Context) (bool, error) {
	metricValue, err := s.executePromQuery(ctx, s.metadata.Query)
	if err != nil {
		return true, fmt.Errorf("failed to execute prometheus query %s: %w", s.metadata.Query, err)
	}
	if metricValue == -1 {
		return true, nil
	}

	if metricValue >= s.metadata.Threshold {
		return true, nil
	}
	return false, nil
}

func (s *prometheusScaler) Close(_ context.Context) error {
	if s.httpClient != nil {
		s.httpClient.CloseIdleConnections()
	}
	return nil
}

func (s *prometheusScaler) IsHealthy(ctx context.Context) (bool, error) {
	cooldownPeriodSeconds := int(math.Ceil(s.cooldownPeriod.Seconds()))
	metricValue, err := s.executePromQuery(
		ctx,
		fmt.Sprintf(uptimeQuery, cooldownPeriodSeconds),
	)
	if err != nil {
		return false, fmt.Errorf("failed to execute prometheus query %s: %w", fmt.Sprintf(uptimeQuery, cooldownPeriodSeconds), err)
	}
	return metricValue == 1, nil
}
