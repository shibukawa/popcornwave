import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { LogRecord, Metric, Snapshot, Trace } from "./types";

export type OtelViewerView = "overview" | "activity" | "metrics";
export type OtelViewerTheme = "dark" | "light";
type Activity = { kind: "trace"; at: string; trace: Trace } | { kind: "log"; at: string; log: LogRecord };
const titles: Record<OtelViewerView, [string, string]> = {
  overview: ["Overview", "Your local telemetry session at a glance."],
  activity: ["Log / Trace", "Compare logs and traces in one chronological timeline."],
  metrics: ["Metrics", "Explore current values and session trends."],
};

export interface OtelViewerProps {
  /** Base URL of the viewer server. Defaults to the current origin. */
  apiEndpoint?: string;
  /** Initial tab. Navigation remains internal and does not change the URL. */
  initialView?: OtelViewerView;
  /** Snapshot refresh interval in milliseconds. */
  pollIntervalMs?: number;
  /** Optional class name added to the component root. */
  className?: string;
  /** Controlled color theme. When omitted, the viewer remembers its own theme. */
  theme?: OtelViewerTheme;
  /** Called when the viewer's theme toggle is used. */
  onThemeChange?: (theme: OtelViewerTheme) => void;
}

export function OtelViewer({
  apiEndpoint = currentOrigin(),
  initialView = "overview",
  pollIntervalMs = 2000,
  className = "",
  theme: controlledTheme,
  onThemeChange,
}: OtelViewerProps) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [view, setView] = useState<OtelViewerView>(initialView);
  const [service, setService] = useState("");
  const [query, setQuery] = useState("");
  const [selectedTraceID, setSelectedTraceID] = useState<string | null>(null);
  const [online, setOnline] = useState(true);
  const [updated, setUpdated] = useState<Date | null>(null);
  const [internalTheme, setInternalTheme] = useState<OtelViewerTheme>(() => storedTheme());
  const theme = controlledTheme ?? internalTheme;

  const refresh = useCallback(async () => {
    try {
      const response = await fetch(endpointURL(apiEndpoint, "/api/snapshot"), { cache: "no-store" });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      setSnapshot(await response.json() as Snapshot);
      setUpdated(new Date());
      setOnline(true);
    } catch { setOnline(false); }
  }, [apiEndpoint]);

  useEffect(() => {
    void refresh();
    if (pollIntervalMs <= 0) return;
    const timer = window.setInterval(() => void refresh(), pollIntervalMs);
    return () => window.clearInterval(timer);
  }, [pollIntervalMs, refresh]);
  useEffect(() => {
    if (controlledTheme == null) window.localStorage.setItem("otelviewer-theme", internalTheme);
  }, [controlledTheme, internalTheme]);

  const toggleTheme = () => {
    const nextTheme = theme === "dark" ? "light" : "dark";
    if (controlledTheme == null) setInternalTheme(nextTheme);
    onThemeChange?.(nextTheme);
  };

  const data = useMemo(() => {
    if (!snapshot || !service) return snapshot;
    return { ...snapshot, traces: snapshot.traces.filter(x => x.Service === service), logs: snapshot.logs.filter(x => x.Service === service), metrics: snapshot.metrics.filter(x => x.Service === service) };
  }, [snapshot, service]);
  const counts = snapshot ?? { traces: [], logs: [], metrics: [] };

  return <div className={`otel-viewer ${className}`.trim()} data-theme={theme}>
    <header><div><span className="mark">O</span><strong>Local OTel Viewer</strong><span className={`live ${online ? "" : "offline"}`}>● {online ? "LIVE" : "OFFLINE"}</span></div>
      <div className="header-actions"><span className="updated">{updated ? `Updated ${updated.toLocaleTimeString()}` : "Waiting for telemetry…"}</span>{(controlledTheme == null || onThemeChange) && <button className="theme-toggle" onClick={toggleTheme} aria-label={`Use ${theme === "dark" ? "light" : "dark"} theme`}>{theme === "dark" ? "☀" : "☾"}</button>}</div></header>
    <main><aside><nav>{(["overview", "activity", "metrics"] as OtelViewerView[]).map(item =>
      <button key={item} className={view === item ? "active" : ""} onClick={() => setView(item)}><span>{item === "activity" ? "Log / Trace" : item[0].toUpperCase() + item.slice(1)}</span>
        {item !== "overview" && <b>{item === "activity" ? counts.traces.length + counts.logs.filter(log => !log.TraceID).length : counts.metrics.length}</b>}</button>)}</nav>
      <section className="endpoint"><label>OTLP/HTTP endpoint</label><code>{apiEndpoint}</code><button onClick={() => void navigator.clipboard.writeText(apiEndpoint)}>Copy endpoint</button></section></aside>
      <section className={`content ${view === "activity" ? "activity-content" : ""}`}>{view === "activity" ? <div className="toolbar activity-toolbar"><h1>{titles[view][0]}</h1><div className="filter compact"><span>⌕</span><input autoFocus aria-label="Filter logs and traces" placeholder="Filter…" value={query} onChange={e => setQuery(e.target.value)} />{query && <button onClick={() => setQuery("")} aria-label="Clear filter">×</button>}</div><ServiceSelect value={service} services={snapshot?.services ?? []} onChange={setService} /></div> : <div className="toolbar"><div><h1>{titles[view][0]}</h1><p>{titles[view][1]}</p></div><ServiceSelect value={service} services={snapshot?.services ?? []} onChange={setService} /></div>}
        {!data ? <Empty title="Loading telemetry…" message="" endpoint={apiEndpoint} /> : view === "overview" ? <Overview data={data} /> : view === "activity" ?
          <ActivityView traces={data.traces} logs={data.logs} correlationLogs={snapshot?.logs ?? data.logs} query={query} selectedTraceID={selectedTraceID} onSelectTrace={setSelectedTraceID} /> : <Metrics metrics={data.metrics} endpoint={apiEndpoint} />}</section></main>
  </div>;
}

function ServiceSelect({ value, services, onChange }: { value: string; services: string[]; onChange: (value: string) => void }) { return <select aria-label="Service" value={value} onChange={e => onChange(e.target.value)}><option value="">All services</option>{services.map(name => <option key={name}>{name}</option>)}</select>; }

function Overview({ data }: { data: Snapshot }) {
  const dropped = Object.values(data.dropped).reduce((sum, value) => sum + value, 0);
  return <><div className="cards"><Card label="Services" value={String(data.services.length)} live /><Card label="Traces" value={String(data.traces.length)} /><Card label="Logs" value={String(data.logs.length)} /><Card label="Dropped" value={String(dropped)} /></div>
    <section className="section-heading"><div><h2>Child process</h2><p>{data.process ? `PID ${data.process.pid} · sampled ${formatTime(data.process.sampledAt)}` : "Available when using otelviewer process"}</p></div></section>
    <div className="process-cards"><Card label="CPU" value={data.process ? `${data.process.cpuPercent.toFixed(1)}%` : "—"} /><Card label="Memory" value={data.process ? formatBytes(data.process.memoryBytes) : "—"} /><Card label="Threads" value={data.process?.threads == null ? "—" : String(data.process.threads)} /><Card label="Open files" value={data.process?.openFiles == null ? "—" : String(data.process.openFiles)} /></div>
    <div className="grid"><div className="panel"><h2>Recent traces</h2><TraceTable traces={data.traces.slice(0, 7)} /></div><div className="panel"><h2>Recent logs</h2>{data.logs.length ? data.logs.slice(0, 7).map((log, index) => <div className="recent-log" key={`${log.Timestamp}-${index}`}><span className="pill">{log.Severity || "LOG"}</span> {log.Body}<div className="attrs">{log.Service} · {formatTime(log.Timestamp)}</div></div>) : <div className="empty compact">No logs received</div>}</div></div></>;
}

function Card({ label, value, live = false }: { label: string; value: string; live?: boolean }) { return <div className="card">{live && <i />}<label>{label}</label><strong>{value}</strong></div>; }

function ActivityView({ traces, logs, correlationLogs, query, selectedTraceID, onSelectTrace }: { traces: Trace[]; logs: LogRecord[]; correlationLogs: LogRecord[]; query: string; selectedTraceID: string | null; onSelectTrace: (id: string | null) => void }) {
  const [listPercent, setListPercent] = useState(38);
  const [selectedStandaloneKey, setSelectedStandaloneKey] = useState<string | null>(null);
  const splitRef = useRef<HTMLDivElement>(null);
  const resize = (clientY: number) => { const rect = splitRef.current?.getBoundingClientRect(); if (!rect) return; setListPercent(Math.max(18, Math.min(72, (clientY - rect.top) / rect.height * 100))); };
  const beginResize = (event: React.PointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    resize(event.clientY);
    const move = (pointerEvent: PointerEvent) => resize(pointerEvent.clientY);
    const stop = () => { window.removeEventListener("pointermove", move); window.removeEventListener("pointerup", stop); window.removeEventListener("pointercancel", stop); };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
  };
  const activities = useMemo(() => {
    const all: Activity[] = [...traces.map(trace => ({ kind: "trace" as const, at: trace.Start, trace })), ...logs.filter(log => !log.TraceID).map(log => ({ kind: "log" as const, at: log.Timestamp, log }))];
    const needle = query.trim().toLocaleLowerCase();
    return all.filter(item => !needle || JSON.stringify(item).toLocaleLowerCase().includes(needle)).sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime());
  }, [traces, logs, query]);
  const selected = traces.find(trace => trace.TraceID === selectedTraceID) ?? null;
  const selectedStandalone = logs.find(log => !log.TraceID && logKey(log) === selectedStandaloneKey) ?? null;
  const correlated = useMemo(() => selected ? correlationLogs.filter(log => log.TraceID === selected.TraceID).sort((a, b) => new Date(a.Timestamp).getTime() - new Date(b.Timestamp).getTime()) : [], [correlationLogs, selected]);
  return <div className="activity-shell">
    <div className="activity-split" ref={splitRef} style={{ gridTemplateRows: `minmax(80px, ${listPercent}fr) 10px minmax(160px, ${100 - listPercent}fr)` }}><section className="panel timeline activity-list" aria-label="Activity list">{!activities.length ? <div className="empty compact"><strong>{query ? "No matching activity" : "No logs or traces yet"}</strong>{query ? "Try another search term." : "Correlated logs appear with their trace."}</div> : activities.map((item, index) => item.kind === "trace" ?
      <button className={`timeline-row ${selectedTraceID === item.trace.TraceID ? "selected" : ""}`} key={`trace-${item.trace.TraceID}`} onClick={() => { setSelectedStandaloneKey(null); onSelectTrace(selectedTraceID === item.trace.TraceID ? null : item.trace.TraceID); }}><span className="timeline-time">{formatTime(item.at)}</span><span className="signal trace-signal">TRACE</span><span className="timeline-main"><strong>{item.trace.Name}</strong><small>{item.trace.TraceID.slice(0, 16)} · {item.trace.Spans.length} spans</small></span><span className="service">{item.trace.Service}</span><span>{item.trace.DurationMS.toFixed(2)} ms</span></button> :
      <button className={`timeline-row ${selectedStandaloneKey === logKey(item.log) ? "selected" : ""}`} key={`log-${item.at}-${index}`} onClick={() => { onSelectTrace(null); setSelectedStandaloneKey(selectedStandaloneKey === logKey(item.log) ? null : logKey(item.log)); }}><span className="timeline-time">{formatTime(item.at)}</span><span className="signal log-signal">{item.log.Severity || "LOG"}</span><span className="timeline-main"><strong>{item.log.Body}</strong><small>{JSON.stringify(item.log.Attributes)}</small></span><span className="service">{item.log.Service}</span><span /></button>)}</section>
      <div className="splitter" role="separator" aria-label="Resize activity list and trace detail" aria-orientation="horizontal" aria-valuemin={18} aria-valuemax={72} aria-valuenow={Math.round(listPercent)} tabIndex={0} onPointerDown={beginResize} onKeyDown={e => { if (e.key === "ArrowUp") setListPercent(value => Math.max(18, value - 3)); if (e.key === "ArrowDown") setListPercent(value => Math.min(72, value + 3)); }}><span /></div>
      <section className="panel trace-detail" aria-label="Activity detail">{selected ? <TraceDetail trace={selected} logs={correlated} /> : selectedStandalone ? <StandaloneLogDetail log={selectedStandalone} /> : <div className="empty detail-empty"><strong>Select a trace or log</strong>Its details will remain visible here.</div>}</section></div></div>;
}

function StandaloneLogDetail({ log }: { log: LogRecord }) { return <div className="detail standalone-log-detail"><div className="detail-heading"><h2>{log.Severity || "LOG"} · {log.Body}</h2><span>{formatTime(log.Timestamp)}</span></div><div className="standalone-log-meta"><span className="service">{log.Service}</span><span>{log.TraceID ? `trace ${log.TraceID}` : "standalone log"}</span></div><div className="json-detail"><h3>Log JSON</h3><pre>{prettyLog(log)}</pre></div></div>; }

function TraceTable({ traces }: { traces: Trace[] }) { if (!traces.length) return <div className="empty compact">No traces received</div>; return <table><thead><tr><th>Trace</th><th>Service</th><th>Duration</th><th>Status</th></tr></thead><tbody>{traces.map(trace => <tr key={trace.TraceID}><td>{trace.Name}<div className="attrs">{trace.TraceID.slice(0, 16)}</div></td><td className="service">{trace.Service}</td><td>{trace.DurationMS.toFixed(2)} ms</td><td className={trace.Status.includes("ERROR") ? "status-error" : ""}>{trace.Status.replace("STATUS_CODE_", "")}</td></tr>)}</tbody></table>; }

function TraceDetail({ trace, logs }: { trace: Trace; logs: LogRecord[] }) {
  const [selectedLog, setSelectedLog] = useState<string | null>(null);
  useEffect(() => setSelectedLog(null), [trace.TraceID]);
  const starts = trace.Spans.map(span => new Date(span.Start).getTime()), ends = trace.Spans.map(span => new Date(span.End).getTime());
  const min = Math.min(...starts), max = Math.max(...ends), range = Math.max(1, max - min);
  const selected = logs.find(log => logKey(log) === selectedLog) ?? null;
  const unmatched = logs.filter(log => !trace.Spans.some(span => span.SpanID === log.SpanID));
  const markerLeft = (timestamp: string) => Math.max(0, Math.min(100, (new Date(timestamp).getTime() - min) / range * 100));
  return <div className="detail"><div className="detail-heading"><h2>{trace.Name} · {trace.Spans.length} spans</h2><span>{logs.length} correlated logs</span></div>
    {unmatched.length > 0 && <div className="trace-marker-lane"><span>Trace</span><div>{unmatched.map(log => <LogMarker key={logKey(log)} log={log} left={markerLeft(log.Timestamp)} selected={selectedLog === logKey(log)} onSelect={setSelectedLog} />)}</div></div>}
    <div className="span-list">{trace.Spans.map(span => { const left = (new Date(span.Start).getTime() - min) / range * 100; const width = Math.max(1, (new Date(span.End).getTime() - new Date(span.Start).getTime()) / range * 100); const spanLogs = logs.filter(log => log.SpanID === span.SpanID); const highlighted = selected?.SpanID === span.SpanID; return <div className={`span ${highlighted ? "highlighted" : ""}`} key={span.SpanID}><div>{span.Name}<div className="attrs">{span.Service}</div></div><div className="span-track"><div className="bar" style={{ marginLeft: `${left}%`, width: `${Math.min(width, 100 - left)}%` }} />{spanLogs.map(log => <LogMarker key={logKey(log)} log={log} left={markerLeft(log.Timestamp)} selected={selectedLog === logKey(log)} onSelect={setSelectedLog} />)}</div><div>{span.DurationMS.toFixed(2)} ms</div></div>; })}</div>
    <div className="log-detail-grid"><div className="correlated-logs"><h3>Correlated logs</h3>{logs.length ? logs.map(log => <button key={logKey(log)} className={selectedLog === logKey(log) ? "selected" : ""} onClick={() => setSelectedLog(logKey(log))}><span>{formatTime(log.Timestamp)}</span><span className="pill">{log.Severity || "LOG"}</span><strong>{log.Body}</strong><small>{log.SpanID ? `span ${log.SpanID.slice(0, 12)}` : "trace level"}</small></button>) : <div className="empty compact">No correlated logs</div>}</div>
      <div className="json-detail"><h3>Log JSON</h3>{selected ? <pre>{prettyLog(selected)}</pre> : <div className="empty compact">Select a log to inspect its JSON</div>}</div></div></div>;
}

function LogMarker({ log, left, selected, onSelect }: { log: LogRecord; left: number; selected: boolean; onSelect: (key: string) => void }) { return <button className={`log-marker ${selected ? "selected" : ""}`} style={{ left: `${left}%` }} title={`${formatTime(log.Timestamp)} ${log.Body}`} aria-label={`Select log at ${formatTime(log.Timestamp)}`} onClick={() => onSelect(logKey(log))} />; }
function logKey(log: LogRecord) { return `${log.Timestamp}:${log.SpanID}:${log.Body}`; }
function prettyLog(log: LogRecord) { let body: unknown = log.Body; try { body = JSON.parse(log.Body); } catch { /* retain plain text */ } return JSON.stringify({ timestamp: log.Timestamp, severity: log.Severity, service: log.Service, traceId: log.TraceID, spanId: log.SpanID || null, body, attributes: log.Attributes }, null, 2); }

function Metrics({ metrics, endpoint }: { metrics: Metric[]; endpoint: string }) {
  const groups = useMemo(() => { const result = new Map<string, Metric[]>(); for (const metric of [...metrics].reverse()) { const key = `${metric.Service}\u0000${metric.Name}\u0000${JSON.stringify(metric.Attributes)}`; result.set(key, [...(result.get(key) ?? []), metric]); } return [...result.values()]; }, [metrics]);
  if (!metrics.length) return <Empty title="No metrics yet" message="Gauges, sums, histograms, and summaries will appear here." endpoint={endpoint} />;
  return <div className="metric-grid">{groups.map((series, index) => { const latest = series[series.length - 1]; return <article className="metric-card" key={`${latest.Name}-${index}`}><div className="metric-title"><div><h2 title={latest.Name}>{latest.Name}</h2><p>{latest.Service} · {latest.Type}{latest.Temporality && ` · ${latest.Temporality.replace("AGGREGATION_TEMPORALITY_", "").toLowerCase()}`}{latest.Unit && ` · ${latest.Unit}`}</p></div><span className="pill">{latest.Type}</span></div><MetricPresentation metrics={series} /><div className="attrs" title={JSON.stringify(latest.Attributes)}>{JSON.stringify(latest.Attributes)}</div></article>; })}</div>;
}

function MetricPresentation({ metrics }: { metrics: Metric[] }) {
  const latest = metrics[metrics.length - 1];
  if (latest.Type === "histogram") return <HistogramDistribution metrics={metrics} />;
  if (latest.Type === "exponential_histogram") return <ExponentialHistogramDistribution metrics={metrics} />;
  if (latest.Type === "summary") return <SummaryStatistics metric={latest} />;
  if (latest.Type === "sum" || latest.Type === "gauge") return <MetricChart metrics={metrics} />;
  return <MetricStatistics metric={latest} />;
}

type HistogramValue = { count?: number; sum?: number; min?: number; max?: number; explicitBounds?: number[]; bucketCounts?: number[] };
function HistogramDistribution({ metrics }: { metrics: Metric[] }) {
  const latest = metrics[metrics.length - 1];
  const latestValue = latest.Value as HistogramValue;
  const bounds = latestValue.explicitBounds ?? [];
  const source = latest.Temporality?.includes("CUMULATIVE") ? [latest] : metrics;
  const counts = source.reduce<number[]>((total, metric) => { const buckets = (metric.Value as HistogramValue).bucketCounts ?? []; return buckets.map((count, index) => (total[index] ?? 0) + count); }, []);
  const total = counts.reduce((sum, count) => sum + count, 0);
  const sum = source.reduce((value, metric) => value + ((metric.Value as HistogramValue).sum ?? 0), 0);
  const minimum = Math.min(...source.map(metric => (metric.Value as HistogramValue).min ?? Number.POSITIVE_INFINITY));
  const maximum = Math.max(...source.map(metric => (metric.Value as HistogramValue).max ?? Number.NEGATIVE_INFINITY));
  return <div className="histogram"><div className="histogram-stats"><Stat label="Count" value={formatValue(total)} /><Stat label="Average" value={total ? formatValue(sum / total) : "—"} /><Stat label="Min" value={Number.isFinite(minimum) ? formatValue(minimum) : "—"} /><Stat label="Max" value={Number.isFinite(maximum) ? formatValue(maximum) : "—"} /></div>
    <div className="histogram-stack" aria-label="Histogram bucket distribution">{counts.map((count, index) => <span key={index} style={{ flexGrow: total ? count / total : 0, background: `hsl(${160 + index * 31 % 170} 65% 55%)` }} title={`${bucketLabel(bounds, index)}: ${count}`} />)}</div>
    <div className="bucket-legend">{counts.map((count, index) => <div key={index}><i style={{ background: `hsl(${160 + index * 31 % 170} 65% 55%)` }} /><span>{bucketLabel(bounds, index)}</span><strong>{formatValue(count)}</strong><small>{total ? `${(count / total * 100).toFixed(1)}%` : "0%"}</small></div>)}</div></div>;
}
function bucketLabel(bounds: number[], index: number) { if (!bounds.length) return `bucket ${index + 1}`; if (index === 0) return `≤ ${formatValue(bounds[0])}`; if (index >= bounds.length) return `> ${formatValue(bounds[bounds.length - 1])}`; return `${formatValue(bounds[index - 1])}–${formatValue(bounds[index])}`; }
function Stat({ label, value }: { label: string; value: string }) { return <div><small>{label}</small><strong>{value}</strong></div>; }
type ExponentialBuckets = { offset?: number; bucketCounts?: number[] };
type ExponentialHistogramValue = { count?: number; sum?: number; min?: number; max?: number; scale?: number; zeroCount?: number; positive?: ExponentialBuckets; negative?: ExponentialBuckets };
type DistributionBucket = { key: string; label: string; count: number };
function ExponentialHistogramDistribution({ metrics }: { metrics: Metric[] }) {
  const latest = metrics[metrics.length - 1];
  const latestValue = latest.Value as ExponentialHistogramValue;
  const scale = latestValue.scale ?? 0;
  const source = (latest.Temporality?.includes("CUMULATIVE") ? [latest] : metrics).filter(metric => ((metric.Value as ExponentialHistogramValue).scale ?? 0) === scale);
  const counts = new Map<string, number>();
  const add = (key: string, count: number) => counts.set(key, (counts.get(key) ?? 0) + count);
  for (const metric of source) {
    const value = metric.Value as ExponentialHistogramValue;
    for (const [index, count] of (value.negative?.bucketCounts ?? []).entries()) add(`n:${(value.negative?.offset ?? 0) + index}`, count);
    if (value.zeroCount) add("z:0", value.zeroCount);
    for (const [index, count] of (value.positive?.bucketCounts ?? []).entries()) add(`p:${(value.positive?.offset ?? 0) + index}`, count);
  }
  const base = Math.pow(2, Math.pow(2, -scale));
  const negatives = [...counts].filter(([key]) => key.startsWith("n:")).sort((a, b) => Number(b[0].slice(2)) - Number(a[0].slice(2))).map(([key, count]) => exponentialBucket(key, count, base));
  const zero = counts.has("z:0") ? [{ key: "z:0", label: "zero", count: counts.get("z:0") ?? 0 }] : [];
  const positives = [...counts].filter(([key]) => key.startsWith("p:")).sort((a, b) => Number(a[0].slice(2)) - Number(b[0].slice(2))).map(([key, count]) => exponentialBucket(key, count, base));
  const buckets = [...negatives, ...zero, ...positives];
  const total = buckets.reduce((sum, bucket) => sum + bucket.count, 0);
  const sum = source.reduce((value, metric) => value + ((metric.Value as ExponentialHistogramValue).sum ?? 0), 0);
  const minimum = Math.min(...source.map(metric => (metric.Value as ExponentialHistogramValue).min ?? Number.POSITIVE_INFINITY));
  const maximum = Math.max(...source.map(metric => (metric.Value as ExponentialHistogramValue).max ?? Number.NEGATIVE_INFINITY));
  return <Distribution title={`Exponential buckets · scale ${scale}`} buckets={buckets} total={total} sum={sum} minimum={minimum} maximum={maximum} />;
}
function exponentialBucket(key: string, count: number, base: number): DistributionBucket { const [sign, rawIndex] = key.split(":"); const index = Number(rawIndex); const low = Math.pow(base, index), high = Math.pow(base, index + 1); return { key, count, label: sign === "n" ? `−${formatCompact(high)}…−${formatCompact(low)}` : `${formatCompact(low)}…${formatCompact(high)}` }; }
function formatCompact(value: number) { return new Intl.NumberFormat(undefined, { maximumSignificantDigits: 3, notation: Math.abs(value) >= 10000 || Math.abs(value) < .001 ? "scientific" : "standard" }).format(value); }
function Distribution({ title, buckets, total, sum, minimum, maximum }: { title: string; buckets: DistributionBucket[]; total: number; sum: number; minimum: number; maximum: number }) { return <div className="histogram"><div className="distribution-title">{title}</div><div className="histogram-stats"><Stat label="Count" value={formatValue(total)} /><Stat label="Average" value={total ? formatValue(sum / total) : "—"} /><Stat label="Min" value={Number.isFinite(minimum) ? formatValue(minimum) : "—"} /><Stat label="Max" value={Number.isFinite(maximum) ? formatValue(maximum) : "—"} /></div><div className="histogram-stack" aria-label={title}>{buckets.map((bucket, index) => <span key={bucket.key} style={{ flexGrow: total ? bucket.count / total : 0, background: distributionColor(index) }} title={`${bucket.label}: ${bucket.count}`} />)}</div><div className="bucket-legend">{buckets.map((bucket, index) => <div key={bucket.key}><i style={{ background: distributionColor(index) }} /><span>{bucket.label}</span><strong>{formatValue(bucket.count)}</strong><small>{total ? `${(bucket.count / total * 100).toFixed(1)}%` : "0%"}</small></div>)}</div></div>; }
function distributionColor(index: number) { return `hsl(${160 + index * 31 % 170} 65% 55%)`; }
type SummaryValue = { count?: number; sum?: number; quantiles?: Array<{ quantile: number; value: number }> };
function SummaryStatistics({ metric }: { metric: Metric }) { const value = metric.Value as SummaryValue; const quantiles = value.quantiles ?? []; const maximum = Math.max(0, ...quantiles.map(item => Math.abs(item.value))); return <div className="summary"><div className="histogram-stats"><Stat label="Count" value={formatValue(value.count ?? 0)} /><Stat label="Sum" value={formatValue(value.sum ?? 0)} /><Stat label="Average" value={value.count ? formatValue((value.sum ?? 0) / value.count) : "—"} /></div><div className="quantiles" aria-label="Summary quantiles">{quantiles.length ? quantiles.map(item => <div key={item.quantile}><span>p{formatValue(item.quantile * 100)}</span><div><i style={{ width: `${maximum ? Math.abs(item.value) / maximum * 100 : 0}%` }} /></div><strong>{formatValue(item.value)}</strong></div>) : <div className="chart-empty">No quantiles reported</div>}</div></div>; }
function MetricStatistics({ metric }: { metric: Metric }) { const value = metric.Value; if (!value || typeof value !== "object") return <div className="metric-value"><strong>{formatValue(value)}</strong>{metric.Unit && <small>{metric.Unit}</small>}</div>; const entries = Object.entries(value).filter(([, entry]) => typeof entry !== "object"); return <div className="metric-value structured">{entries.map(([label, entry]) => <div key={label}><small>{label}</small><strong>{formatValue(entry)}</strong>{label === "sum" && metric.Unit && <em>{metric.Unit}</em>}</div>)}</div>; }

function MetricChart({ metrics }: { metrics: Metric[] }) { const values = metrics.map(metricNumber).filter((value): value is number => value != null); if (!values.length) return <div className="chart-empty">Structured value · {formatValue(metrics[metrics.length - 1].Value)}</div>; const min = Math.min(...values), max = Math.max(...values), spread = Math.max(max - min, Math.abs(max) * .05, 1); const points = values.map((value, i) => `${values.length === 1 ? 50 : i / (values.length - 1) * 100},${92 - (value - min) / spread * 76}`).join(" "); return <div className="chart"><div className="chart-value">{formatValue(metrics[metrics.length - 1].Value)}</div><svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-label="Metric time series"><polyline points={points} fill="none" vectorEffect="non-scaling-stroke" /></svg><span>{formatValue(min)}</span><span>{formatValue(max)}</span></div>; }
function metricNumber(metric: Metric) { return typeof metric.Value === "number" ? metric.Value : null; }
function Empty({ title, message, endpoint }: { title: string; message: string; endpoint: string }) { return <div className="panel empty"><strong>{title}</strong>{message}<br /><br /><code>OTEL_EXPORTER_OTLP_ENDPOINT={endpoint}</code></div>; }
function endpointURL(endpoint: string, path: string) { return `${endpoint.trim().replace(/\/+$/, "")}${path}`; }
function currentOrigin() { return typeof window === "undefined" ? "" : window.location.origin; }
function storedTheme(): OtelViewerTheme { return typeof window !== "undefined" && window.localStorage.getItem("otelviewer-theme") === "light" ? "light" : "dark"; }
function formatTime(value: string) { return value && value !== "0001-01-01T00:00:00Z" ? new Date(value).toLocaleTimeString() : "—"; }
function formatValue(value: unknown) { return typeof value === "number" ? new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 }).format(value) : typeof value === "object" ? JSON.stringify(value) : String(value ?? ""); }
function formatBytes(value: number) { if (!value) return "0 B"; const units = ["B", "KB", "MB", "GB"]; const exponent = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1); return `${(value / 1024 ** exponent).toFixed(exponent ? 1 : 0)} ${units[exponent]}`; }
export default OtelViewer;
