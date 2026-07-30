export interface Span {
  TraceID: string;
  SpanID: string;
  ParentSpanID: string;
  Name: string;
  Service: string;
  Kind: string;
  Status: string;
  Start: string;
  End: string;
  DurationMS: number;
  Attributes: Record<string, unknown>;
  Events: string[];
}

export interface Trace {
  TraceID: string;
  Service: string;
  Name: string;
  Start: string;
  DurationMS: number;
  Status: string;
  Spans: Span[];
}

export interface LogRecord {
  Timestamp: string;
  Severity: string;
  Body: string;
  Service: string;
  TraceID: string;
  SpanID: string;
  Attributes: Record<string, unknown>;
}

export interface Metric {
  Timestamp: string;
  Name: string;
  Description: string;
  Unit: string;
  Type: string;
  Service: string;
  Temporality?: string;
  Monotonic?: boolean;
  Value: unknown;
  Attributes: Record<string, unknown>;
}

export interface Snapshot {
  generatedAt: string;
  services: string[];
  traces: Trace[];
  logs: LogRecord[];
  metrics: Metric[];
  dropped: Record<string, number>;
  process?: ProcessHealth;
}

export interface ProcessHealth {
  pid: number;
  cpuPercent: number;
  memoryBytes: number;
  threads?: number;
  openFiles?: number;
  readBytes?: number;
  writtenBytes?: number;
  sampledAt: string;
}
