package hollander

import (
	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	NbReq            prometheus.Counter
	NbReq2xx         prometheus.Counter
	NbReq4xx         prometheus.Counter
	NbReq5xx         prometheus.Counter
	Latency2xxMillis prometheus.Summary
	NbCurrentConns   prometheus.Gauge
}

// https://youtrack.wildberries.ru/articles/SAPI-A-60/Metriki
func NewHttpMetrics(ns, methodName string) HTTPMetrics {
	return HTTPMetrics{
		NbReq:            newCounter(ns, "http_nb_req", methodName),
		NbReq2xx:         newCounter(ns, "http_nb_req_2xx", methodName),
		NbReq4xx:         newCounter(ns, "http_nb_req_4xx", methodName),
		NbReq5xx:         newCounter(ns, "http_nb_req_5xx", methodName),
		Latency2xxMillis: newSummary(ns, "http_latency_2xx_ms", methodName),
		NbCurrentConns:   newGauge(ns, "http_nb_current_conns", methodName),
	}
}
