# Known Issues & Improvements

## 1. Current Known Issues

### 1.1 Link Checker False Positives

**Issue**: Some sites block automated tools (403 Forbidden)  
**Affected Sites**: Medium.com, StackOverflow.com, X/Twitter  
**Impact**: Links reported as inaccessible may actually work in browsers  
**Workaround**: Manually verify "broken" links in browser  
**Success Rate**: ~97% on most sites  

### 1.2 Performance with Large Link Counts

**Issue**: Pages with >100 links take longer to check  
**Recommendation**: Use async mode for pages with many links  

## 2. Critical Issues (Must Fix for Production)

### 2.1 SSRF Protection

**Problem**: Application can be used to scan internal networks  
**Risk**: Can analyze localhost/private IPs (10.x.x.x, 192.168.x.x, 127.0.0.1)  
**Impact**: Potential security risk in production environments  
**Fix Required**: Blacklist private IP ranges before fetching  

### 2.2 Rate Limiting

**Problem**: No protection against abuse  
**Risk**: Unlimited requests per IP, easy to DoS  
**Fix Required**: Implement token bucket or sliding window (10 req/min per IP)  

### 2.3 Authentication

**Problem**: API is completely open
**Risk**: Anyone can use the service without restriction
**Fix**: Add API key authentication or JWT tokens

### 2.4 Cost Considerations for Public Deployment

**Warning**: Deploying this service publicly can become expensive  

**Factors**:

- **Many users = many jobs**: Each analysis spawns async jobs with worker pools
- **Cache growth**: Popular sites cached, but unique URLs grow cache size unbounded
- **Memory usage**: In-memory job storage + LRU cache scales with traffic
- **Redis costs**: Multi-instance deployments with Redis cache can be costly at scale
- **Bandwidth**: Fetching full HTML pages + checking all links = high egress traffic
- **Compute**: Link checking is CPU/network intensive (20+ workers per job)

**Recommendations**:

- Implement rate limiting (see 2.2) to control resource usage per user
- Set aggressive cache TTLs and size limits
- Use Redis with eviction policies (LRU/LFU) to cap memory
- Monitor costs closely in cloud environments (bandwidth, compute, storage)
- Consider per-user quotas or paid tiers for high-volume usage
- Offload link checking to background workers with job queues for cost-effective scaling

**Development vs Production**: Demo mode runs fine locally, but production at scale requires cost management strategy.

**Cost Estimation** (example with 5,000 analyses/day):

```
Compute:    5,000 × 6s/analysis = 8.3 CPU-hours/day × $0.05 = $13/month
Bandwidth:  5,000 × 0.5 MB = 2.5 GB/day × $0.09/GB = $7/month
Redis:      Optional, ~$30-50/month
Total:      $20-70/month (scales linearly with traffic)
```

**Your formula**: `monthly_cost = (analyses_per_day × 6s / 3600 × $0.05 × 30) + (analyses_per_day × 0.5MB / 1024 × $0.09 × 30) + redis`

At 100K analyses/day: ~$400-1,400/month. Cache (90% hit) cuts costs by 50-70%.

## 3. High Priority

### 3.1 Integration Tests

**Gap**: Unit test coverage is good (86-93%), integration tests are minimal  
**Need**: Full API tests, Redis tests, end-to-end scenarios, error handling  

### 3.2 Job Storage

**Problem**: Async jobs stored in memory  
**Issues**: Jobs lost on restart, not shared across instances, no cleanup  
**Fix**: Move job storage to Redis for persistence and multi-instance support  

### 3.3 Connection Pooling

**Problem**: Creates new HTTP client for each link check  
**Impact**: Higher latency, more resource usage  
**Fix**: Shared HTTP client with connection pool and keep-alive  

## 4. Medium Priority

### 4.1 Observability Improvements

**Current State**: Infrastructure exists (OTel, Jaeger, Prometheus, Grafana) but largely unused

**Critical Gaps**:

1. **Metrics not recorded**: Defined but never called
   - ❌ `RecordCacheHit/Miss` - Not called in cache implementations
   - ❌ `RecordLinksChecked` - Not called in link checker
   - ❌ `RecordAnalysisDuration` - Not called in analyzer
   - ✅ `RecordHTTPRequest` - Only HTTP metrics work
2. **Metrics not injected**: Created in main but not passed to cache/analyzer/link checker
3. **No error tracking**: No metrics for 4xx/5xx, timeouts, parse failures, cache errors
4. **Dashboard incomplete**: Missing error panels, cache breakdown (L1/L2), link check metrics, queue depth, percentiles (p99)
5. **Tracing too shallow**: Missing spans for collectors, individual link checks, Redis ops, HTTP fetches
6. **No alerting**: No Prometheus alert rules for high errors, slow requests, low cache hit rate, queue backlog
7. **Queue gauge not implemented**: `queueSize` defined but callback never registered
8. **No histogram buckets**: Using defaults, not optimized for actual latencies (10ms-30s)

**Quick fixes** (< 1 day):
- Add metric recording calls in cache, analyzer, link checker
- Pass metrics object to components
- Add error counter metric
- Configure histogram buckets
- Implement queue gauge callback

**Impact**: Can only see HTTP traffic, blind to core operations. Can't debug production issues.  

### 4.2 HTML Parser Limitations

**Known Limitation**: Streaming parser doesn't execute JavaScript  
**Impact**: SPAs (React, Vue, Angular) show incomplete results, AJAX-loaded links not found  
**Options**: Document limitation OR add optional headless browser mode (heavy)  

### 4.3 Cache Key Design

**Issue**: Cache key is URL only, doesn't include options  
**Impact**: Same URL with different options (checkLinks, maxLinks) shares cache entry  
**Fix**: Include options in cache key hash  

## 5. Architecture Limitations

### 5.1 Max Page Size

**Limit**: 10MB / 1M tokens (configurable)  
**Impact**: Very large pages may be truncated  
**Rationale**: Prevents memory exhaustion  

### 5.2 No Authentication Support

**Limitation**: Cannot analyze pages behind login  
**Impact**: Cannot analyze authenticated content  

### 5.3 Subdomains Treated as External

**Behavior**: `blog.example.com` → `example.com` considered external  
**Rationale**: Simple heuristic, but may not match user expectations  

## 6. Low Priority Improvements

- Robots.txt respect
- Sitemap discovery and parsing
- Enhanced SEO analysis (Open Graph, Twitter Cards, Schema.org)
- Accessibility checks (WCAG compliance)
- Kubernetes deployment manifests
- CI/CD pipeline configuration
- Enhanced error messages
- Request/response logging for audit

## 7. Summary

The application meets all specified requirements. However, **critical security issues (2.1 SSRF, 2.2 Rate Limiting) must be addressed before production deployment**.

Most improvements are additive and don't require refactoring. The architecture provides a solid foundation for extension.
