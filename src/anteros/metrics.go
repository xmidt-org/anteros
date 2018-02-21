package main

import (
	"github.com/Comcast/webpa-common/xmetrics"
	"github.com/go-kit/kit/metrics"
)

const (
	ResponseReceivedWebPA = "response_received_webpa_count"
	ResponseReceivedXMiDT = "response_received_xmidt_count"
	ResponseUsedWebPA     = "response_used_webpa_count"
	ResponseUsedXMiDT     = "response_used_xmidt_count"
)

type Metrics struct {
	ResponseReceivedWebPA metrics.Counter
	ResponseReceivedXMiDT metrics.Counter
	ResponseUsedWebPA     metrics.Counter
	ResponseUsedXMiDT     metrics.Counter
}

func GetMetrics() []xmetrics.Metric {
	return []xmetrics.Metric{
		xmetrics.Metric{
			Name: ResponseReceivedWebPA,
			Help: "Count of the number of WebPA Responses returned",
			Type: "counter",
		},
		xmetrics.Metric{
			Name: ResponseReceivedXMiDT,
			Help: "Count of the number of XMiDT Responses returned",
			Type: "counter",
		},
		xmetrics.Metric{
			Name: ResponseUsedWebPA,
			Help: "Count of the number of WebPA Responses used as final response",
			Type: "counter",
		},
		xmetrics.Metric{
			Name: ResponseUsedXMiDT,
			Help: "Count of the number of XMiDT Responses used as final response",
			Type: "counter",
		},
	}
}

func AddMetrics(registry xmetrics.Registry) (m Metrics) {
	for _, metric := range GetMetrics() {
		switch metric.Name {
		case ResponseReceivedWebPA:
			m.ResponseReceivedWebPA = registry.NewCounter(metric.Name)
		case ResponseReceivedXMiDT:
			m.ResponseReceivedXMiDT = registry.NewCounter(metric.Name)
		case ResponseUsedWebPA:
			m.ResponseUsedWebPA = registry.NewCounter(metric.Name)
		case ResponseUsedXMiDT:
			m.ResponseUsedXMiDT = registry.NewCounter(metric.Name)
		}
	}
	
	return
}
