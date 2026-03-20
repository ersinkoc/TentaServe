# Benchmarks

Results from `go test -bench=. -benchtime=2s ./test/bench/` on AMD Ryzen 9 9950X3D (Windows 11, Go 1.22).

## Proxy

| Benchmark | ops/sec | latency | allocs/op | bytes/op |
|-----------|---------|---------|-----------|----------|
| Passthrough | ~5,600/s | 178us | 146 | 13.6KB |
| REST to GraphQL | ~5,000/s | 201us | 184 | 17.8KB |
| GraphQL to REST | ~5,400/s | 185us | 171 | 17.1KB |

## Cache

| Benchmark | ops/sec | latency | allocs/op | bytes/op |
|-----------|---------|---------|-----------|----------|
| Cache Hit | ~3.6M/s | 359ns | 6 | 608B |
| Cache Miss | ~20M/s | 66ns | 2 | 24B |
| Cache Concurrent | ~3M/s | 397ns | 6 | 614B |

## Rate Limiter

| Benchmark | ops/sec | latency | allocs/op | bytes/op |
|-----------|---------|---------|-----------|----------|
| Token Bucket | ~206M/s | 5.9ns | 0 | 0B |
| Token Bucket Concurrent | ~1B/s | 0.57ns | 0 | 0B |

## DataLoader

| Benchmark | ops/sec | latency | allocs/op | bytes/op |
|-----------|---------|---------|-----------|----------|
| Batch (8 goroutines) | ~3,500/s | 282us | 7 | 302B |
| Individual (sequential) | ~3.2M/s | 387ns | 10 | 279B |

## Key Observations

- **Rate limiter** is lock-free (atomic CAS) with zero allocations - ~206M ops/sec single-threaded
- **Cache** serves hits in ~360ns with sharded locking for concurrent access
- **Proxy passthrough** adds ~180us overhead per request including HTTP round-trip to mock backend
- **Protocol translation** adds ~20-25us overhead on top of passthrough for query building and response unwrapping
- **DataLoader batching** amortizes N+1 queries effectively with sub-millisecond batch dispatch
