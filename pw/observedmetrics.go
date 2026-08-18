package pw

import (
	"github.com/shibukawa/popcornweb/contrib/otel/metric"
	"github.com/shibukawa/popcornweb/pwdatabase"
	"github.com/shibukawa/popcornweb/pwruntime"
)

// registerObservedMetrics registers the instruments that are read at collection
// rather than recorded on a request path.
//
// They are registered here, once per startup, rather than where the value lives,
// because a callback per store or per pool would leave instruments behind
// pointing at a resource a later configuration replaced. Each callback reads the
// current set instead, so a database opened after this call still reports.
//
// Nothing here runs on a request. The cost of the whole group is one callback
// per instrument per collection interval.
func registerObservedMetrics(config ObservabilityConfig, provider *metric.Provider, metrics *pwruntime.Metrics) {
	if provider == nil || metrics == nil {
		return
	}
	meter := provider.Meter(pwruntime.MetricScope)
	if config.Metrics.Cache {
		pwruntime.RegisterCacheMetrics(meter)
	}
	if config.Metrics.DB {
		pwruntime.RegisterDatabaseMetrics(meter, databasePools)
	}
}

// databasePools names every configured pool for the connection instruments.
//
// A configuration with one database reports it as the default group rather than
// as an unnamed series, because a metric with no pool attribute cannot later gain
// one without splitting the series it already wrote.
func databasePools() []pwruntime.NamedPool {
	connections := pwdatabase.Connections()
	if connections == nil {
		db, _ := pwdatabase.Default()
		if db == nil {
			return nil
		}
		return []pwruntime.NamedPool{{Name: "default", DB: db}}
	}
	pools := make([]pwruntime.NamedPool, 0, connections.Count())
	for _, connection := range connections.Connections() {
		// A native engine bypasses database/sql and has no pool statistics; it
		// reports nothing rather than zeros that would read as an idle pool.
		if connection == nil || connection.DB == nil {
			continue
		}
		name := connection.Label
		if name == "" {
			name = connection.Group
		}
		pools = append(pools, pwruntime.NamedPool{Name: name, DB: connection.DB})
	}
	return pools
}
