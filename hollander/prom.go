package hollander

import "github.com/prometheus/client_golang/prometheus"

func newCounter(ns, name, method string) prometheus.Counter {

	var labels prometheus.Labels
	if method != "" {
		labels = map[string]string{
			"method": method,
		}
	}

	c := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: ns,
			//Subsystem: serviceName,
			Name:        name,
			ConstLabels: labels,
		})
	prometheus.MustRegister(c)
	return c
}

func newGauge(ns, name, method string) prometheus.Gauge {

	var labels prometheus.Labels
	if method != "" {
		labels = map[string]string{
			"method": method,
		}
	}

	g := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: ns,
			//Subsystem: serviceName,
			Name:        name,
			ConstLabels: labels,
		})
	prometheus.MustRegister(g)
	return g
}

func newSummary(ns, name, method string) prometheus.Summary {

	var labels prometheus.Labels
	if method != "" {
		labels = map[string]string{
			"method": method,
		}
	}

	s := prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: ns,
			//Subsystem: serviceName,
			Name:        name,
			ConstLabels: labels,
		})
	prometheus.MustRegister(s)
	return s
}
