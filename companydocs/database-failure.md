# Database Failure

## Symptoms
- "connection refused" or "connection pool exhausted" errors
- Increased query latency (>1000ms)
- Application pods restarting due to health check failures
- Transactions timing out

## Root Causes
- Connection pool saturation (max connections reached)
- Database replica lag or primary failover
- Disk space exhaustion on the database server
- Long-running queries blocking connection slots

## Action Items
1. Scale up database replicas or increase connection pool limits
2. Adjust connection limits in environment variables
3. Check for long-running queries and kill if necessary
4. Verify disk space on database persistent volumes
5. Review connection pool metrics in monitoring dashboard
