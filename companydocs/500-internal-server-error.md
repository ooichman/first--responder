# 500 Internal Server Error

## Symptoms
- HTTP 500 responses from the service
- Panic stack traces in container logs
- Downstream services returning errors

## Root Causes
- Database connection timeouts or pool exhaustion
- Broken downstream microservices
- Unhandled panic states in application code
- Resource limits exceeded (CPU/Memory)

## Action Items
1. Check database connection pools and active connections
2. Verify upstream/downstream service health
3. Restart the affected pod if in a crash loop
4. Review recent deployments for regressions
5. Check resource utilization (kubectl top pod)
