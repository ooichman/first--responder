# First Responder AI Agent

A containerized, K3s-deployed "First Responder" system that automates error log analysis and troubleshooting using a local vLLM agent.

## 1. Architecture
This system utilizes a microservices approach: a Go-based API handles incoming logs and coordinates with a locally deployed vLLM engine for inference and autonomous tool execution.



## 2. vLLM Inference Engine Deployment

### Prerequisites
* **K3s Cluster:** Running on a KVM virtual machine.
* **Resources:** At least 12GB RAM, 6 vCPUs, and 30GB disk space.
* **CPU Flag Pass-through:** Ensure the VM configuration uses `<cpu mode='host-passthrough'/>` to expose AVX2 instructions to the cluster.

### Persistent Storage
Define a persistent volume claim to cache model weights:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: vllm-model-cache
  namespace: first-responder
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 20Gi
```

## 3. vLLM Inference Engine
Deploy the engine with shared memory (/dev/shm) and tool-calling enabled to ensure compatibility with your Go application:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-server
  namespace: first-responder
spec:
  replicas: 1
  template:
    spec:
      volumes:
        - name: dshm
          emptyDir: {medium: Memory, sizeLimit: "2Gi"}
      containers:
        - name: vllm-engine
          image: public.ecr.aws/q9t5s3a7/vllm-cpu-release-repo:latest
          args:
            - "--model", "Qwen/Qwen2.5-1.5B-Instruct"
            - "--max-model-len", "2048"
            - "--port", "8000"
            - "--tensor-parallel-size", "1"
            - "--gpu-memory-utilization", "0.7"
            - "--enable-auto-tool-choice"
            - "--tool-call-parser", "hermes"
          env:
            - name: VLLM_TARGET_DEVICE, value: "cpu"
          volumeMounts:
            - mountPath: /dev/shm, name: dshm
```


## 4.Application deployment

For the application we already have the YAML file ready in the deployment directory :

```bash
kubectl apply -f deployment/fr-app/deployment.yaml
kubectl apply -f deployment/fr-app/ingress-tls-secret.yaml  
kubectl apply -f deployment/fr-app/ingress.yaml
kubectl apply -f deployment/fr-app/service.yaml
```
**NOTE**

Do not forget to update the TLS certificate and the ingress hostname to your environment


##### Go API Backend
Ensure the environment variable LLM_API_BASE points to http://vllm-service.first-responder.svc.cluster.local:8000/v1.

## 5. API Testing

Analyze a Log
Use `curl` to send a log entry to the endpoint:

```bash
curl -X POST [https://first-responder.apps.k3s-dom.local/analyze](https://first-responder.apps.k3s-dom.local/analyze) \
   -H "Content-Type: application/json" \
   -d '{"log": "Panic exception error: 500 downstream DB connection timed out"}'
```

##### Expected Output
```json
JSON
{
  "summary": "Downstream database connection times out after reaching the deadline",
  "confidence_score": 90,
  "action_items": [
    "Review application logic related to database connections",
    "Check configuration settings for timeouts and resource limits"
  ]
}
```

## 6. Troubleshooting Guide

| Issue | Root Cause | Fix |
| :--- | :--- | :--- |
| Pod Crash (OOM)   | Memory limit exceeded   | Reduce --gpu-memory-utilization to 0.5 or 0.6.
| Illegal Instruction | Missing CPU features  |  Ensure host-passthrough is set in VM configuration.
| Tool Calling 400    | Missing Parser  | Ensure --enable-auto-tool-choice and --tool-call-parser hermes are set.
| Engine Init Fail    | Shared Memory limit | Ensure emptyDir with medium: Memory is mounted to /dev/shm.
