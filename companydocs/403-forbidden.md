# 403 Forbidden

## Symptoms
- HTTP 403 responses on authenticated endpoints
- "Permission denied" or "Access denied" in logs
- Users unable to access previously available resources

## Root Causes
- Misconfigured RBAC policy or ClusterRole bindings
- Expired JWT token or service account credentials
- Incorrect API gateway routing headers
- Network policy blocking inter-service communication

## Action Items
1. Verify service account roles and ClusterRoleBindings
2. Refresh identity tokens and check expiration times
3. Audit network policies for unintended blocks
4. Check API gateway configuration for header forwarding
5. Review recent RBAC changes in the cluster
