# Out of Memory (OOMKilled)

## Symptoms
- Pod status shows "OOMKilled" in kubectl describe
- Container restarts with exit code 137
- "memory cgroup out of memory" in kernel logs
- Sudden pod termination without graceful shutdown

## Root Causes
- Container memory limit set too low for the workload
- Memory leak in application code
- Unbounded cache or buffer growth
- Large payload processing without streaming

## Action Items
1. Increase memory limits in the pod spec
2. Investigate memory leaks with profiling tools (pprof)
3. Optimize application memory usage and add bounds
4. Enable memory-aware garbage collection tuning
5. Consider horizontal scaling instead of vertical
