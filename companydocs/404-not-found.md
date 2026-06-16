# 404 Not Found

## Symptoms
- HTTP 404 responses for known endpoints
- "route not found" or "no matching handler" in logs
- Client applications receiving unexpected errors

## Root Causes
- Misconfigured ingress rules or virtual service routes
- Deleted or renamed API endpoints after a deployment
- Incorrect service mesh routing configuration
- DNS resolution failures for internal services

## Action Items
1. Verify ingress rules and path configurations
2. Check service discovery and DNS resolution
3. Confirm the resource path exists in the target service
4. Review recent deployment changes for route modifications
5. Validate service mesh VirtualService definitions
