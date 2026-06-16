# Timeout Error

## Symptoms
- "context deadline exceeded" in application logs
- HTTP 504 Gateway Timeout responses
- Requests hanging before failing
- Increased error rates during peak traffic

## Root Causes
- Upstream service not responding within configured deadline
- Network policies or firewall rules causing packet drops
- Target service is resource-starved (CPU throttled)
- DNS resolution delays in the cluster

## Action Items
1. Increase timeout values in client configuration
2. Check network policies between source and target services
3. Verify the target service has adequate CPU/memory resources
4. Inspect DNS resolution times (coredns logs)
5. Check for circuit breaker activation in service mesh
