package pwruntime

import (
	"database/sql"

	"github.com/shibukawa/popcornweb/contrib/otel"
	"github.com/shibukawa/popcornweb/contrib/otel/metric"
)

// RegisterCacheMetrics registers the data result cache observables.
//
// The counters already exist on every store, incremented on the lookup path, and
// this reads them at collection instead of recording a second time. It is one
// registration for every store rather than one per store, because the set is
// rebuilt whenever its configuration fingerprint changes and a per-store
// registration would leave an instrument behind pointing at a store nobody uses.
//
// The result is an attribute of a closed set rather than four instruments, so a
// reader dividing hits by the total finds both terms under one name. No ratio is
// computed here: a quotient without its denominator cannot tell a cache that is
// working from one nothing is eligible for.
func RegisterCacheMetrics(meter *metric.Meter) {
	if meter == nil {
		return
	}
	meter.ObservableCounter("pw.data_cache.operations", "{operation}",
		"Data result cache lookups, by store and result.", func() []metric.Observation {
			set := cacheStores()
			observations := make([]metric.Observation, 0, len(set.names)*4)
			for _, name := range set.names {
				store := set.stores[name]
				if store == nil {
					continue
				}
				stats := store.Stats()
				observations = append(observations,
					cacheObservation(name, CacheResultHit, stats.Hits),
					cacheObservation(name, CacheResultMiss, stats.Misses),
					cacheObservation(name, CacheResultStaleHit, stats.StaleHits),
					cacheObservation(name, CacheResultCoalesced, stats.Coalesced),
				)
			}
			return observations
		})
	meter.ObservableGauge("pw.data_cache.entries", "{entry}",
		"Entries held by each data result cache.", func() []metric.Observation {
			set := cacheStores()
			observations := make([]metric.Observation, 0, len(set.names))
			for _, name := range set.names {
				store := set.stores[name]
				if store == nil {
					continue
				}
				observations = append(observations, metric.Observation{
					Attributes: []otel.Attribute{otel.String("pw.cache.store", name)},
					Value:      float64(store.Stats().Entries),
				})
			}
			return observations
		})
}

func cacheObservation(store, result string, value int64) metric.Observation {
	return metric.Observation{
		Attributes: []otel.Attribute{
			otel.String("pw.cache.store", store),
			otel.String("pw.cache.result", result),
		},
		Value: float64(value),
	}
}

// RegisterDatabaseMetrics registers the connection pool observables.
//
// They matter because an exhausted pool makes every request slow with no
// statement being slow, which is the one failure the per-statement duration
// histogram cannot show.
//
// pools is read at collection rather than captured, so a process that opens its
// databases after this registration still reports them.
func RegisterDatabaseMetrics(meter *metric.Meter, pools func() []NamedPool) {
	if meter == nil || pools == nil {
		return
	}
	meter.ObservableUpDownCounter("db.client.connection.count", "{connection}",
		"Connections held by each pool, by state.", func() []metric.Observation {
			named := pools()
			observations := make([]metric.Observation, 0, len(named)*2)
			for _, pool := range named {
				if pool.DB == nil {
					continue
				}
				stats := pool.DB.Stats()
				observations = append(observations,
					connectionObservation(pool.Name, "used", float64(stats.InUse)),
					connectionObservation(pool.Name, "idle", float64(stats.Idle)),
				)
			}
			return observations
		})
	meter.ObservableUpDownCounter("db.client.connection.max", "{connection}",
		"Maximum open connections each pool allows.", func() []metric.Observation {
			named := pools()
			observations := make([]metric.Observation, 0, len(named))
			for _, pool := range named {
				if pool.DB == nil {
					continue
				}
				observations = append(observations, poolObservation(pool.Name, float64(pool.DB.Stats().MaxOpenConnections)))
			}
			return observations
		})
	// WaitCount is every wait rather than only the ones that timed out, so this
	// is named for what it counts. The specification's connection.timeouts is
	// deliberately absent: database/sql does not distinguish them, and reporting
	// waits under that name would overstate a failure that may not have happened.
	meter.ObservableCounter("pw.db.connection.waits", "{wait}",
		"Times a caller waited for a connection, since process start.", func() []metric.Observation {
			named := pools()
			observations := make([]metric.Observation, 0, len(named))
			for _, pool := range named {
				if pool.DB == nil {
					continue
				}
				observations = append(observations, poolObservation(pool.Name, float64(pool.DB.Stats().WaitCount)))
			}
			return observations
		})
	// The specification wants a wait-time histogram and database/sql offers only
	// a cumulative total, so the distribution was never recorded and cannot be
	// recovered. This answers whether the pool waits at all, not how badly any
	// one caller did.
	meter.ObservableCounter("pw.db.connection.wait.time", "s",
		"Time spent waiting for a connection, since process start.", func() []metric.Observation {
			named := pools()
			observations := make([]metric.Observation, 0, len(named))
			for _, pool := range named {
				if pool.DB == nil {
					continue
				}
				observations = append(observations, poolObservation(pool.Name, pool.DB.Stats().WaitDuration.Seconds()))
			}
			return observations
		})
}

// NamedPool is one connection pool and the label a metric groups it by.
type NamedPool struct {
	Name string
	DB   *sql.DB
}

func connectionObservation(pool, state string, value float64) metric.Observation {
	return metric.Observation{
		Attributes: []otel.Attribute{
			otel.String("db.client.connection.pool.name", pool),
			otel.String("db.client.connection.state", state),
		},
		Value: value,
	}
}

func poolObservation(pool string, value float64) metric.Observation {
	return metric.Observation{
		Attributes: []otel.Attribute{otel.String("db.client.connection.pool.name", pool)},
		Value:      value,
	}
}
