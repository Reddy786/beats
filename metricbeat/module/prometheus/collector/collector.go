package collector

import (
	"fmt"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/metricbeat/helper"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/elastic/beats/metricbeat/mb/parse"
	"github.com/elastic/beats/metricbeat/module/prometheus"
)

const (
	defaultScheme = "http"
	defaultPath   = "/metrics"
)

var (
	hostParser = parse.URLHostParserBuilder{
		DefaultScheme: defaultScheme,
		DefaultPath:   defaultPath,
		PathConfigKey: "metrics_path",
	}.Build()
)

func init() {
	if err := mb.Registry.AddMetricSet("prometheus", "collector", New, hostParser); err != nil {
		panic(err)
	}
}

type MetricSet struct {
	mb.BaseMetricSet
	http      *helper.HTTP
	namespace string
}

func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	logp.Warn("BETA: The prometheus collector metricset is beta")

	config := struct {
		Namespace string `config:"namespace" validate:"required"`
	}{}
	err := base.Module().UnpackConfig(&config)
	if err != nil {
		return nil, err
	}

	return &MetricSet{
		BaseMetricSet: base,
		http:          helper.NewHTTP(base),
		namespace:     config.Namespace,
	}, nil
}

func (m *MetricSet) Fetch() ([]common.MapStr, error) {

	resp, err := m.http.FetchResponse()
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	families, err := prometheus.GetMetricFamiliesFromResponse(resp)

	if err != nil {
		return nil, fmt.Errorf("Unable to decode response from prometheus endpoint")
	}

	eventList := map[string]common.MapStr{}

	for _, family := range families {
		promEvents := GetPromEventsFromMetricFamily(family)

		for _, promEvent := range promEvents {
			if _, ok := eventList[promEvent.labelHash]; !ok {
				eventList[promEvent.labelHash] = common.MapStr{}

				// Add labels
				if len(promEvent.labels) > 0 {
					eventList[promEvent.labelHash]["label"] = promEvent.labels
				}

			}

			eventList[promEvent.labelHash][promEvent.key] = promEvent.value

		}
	}

	// Converts hash list to slice
	events := []common.MapStr{}
	for _, e := range eventList {
		e["_namespace"] = m.namespace
		events = append(events, e)
	}

	return events, err
}
