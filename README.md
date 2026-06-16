# First Responder AI Agent

A containerized, K3s-deployed "First Responder" system that automates error log analysis and troubleshooting using a local vLLM agent.

## 1. Architecture
This system utilizes a microservices approach: a Go-based API handles incoming logs and coordinates with a locally deployed vLLM engine for inference and autonomous tool execution.



## 2. Agent Logic

The agent uses an LLM to "reason" about the error. It is equipped with at least one Tool (a mock function that retrieves "Company Troubleshooting Docs") to help it decide on a solution.

**Flow:**
1. The user sends a raw error log to the `/analyze` endpoint.
2. The Go API forwards the log to the LLM with a system prompt and a registered tool (`get_company_docs`).
3. The LLM reasons about the error and may invoke the tool to retrieve relevant internal troubleshooting documentation.
4. If the tool is called, its result is fed back to the LLM for a second reasoning pass.
5. The LLM returns a structured JSON response containing a summary, confidence score, and recommended action items.

**Tool — `get_company_docs`:**
A mock function simulating access to internal company troubleshooting documentation. It performs keyword-based matching (e.g., `500`, `403`, `db`) and returns relevant remediation guidance to the LLM.

## 3. vLLM Inference Engine Deployment

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

## 4. vLLM Inference Engine
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


## 5. Application Deployment (Kustomize)

All Kubernetes manifests are managed with [Kustomize](https://kustomize.io/). The layout:

```
deployment/
├── base/                    # Shared resources (Deployment, Service, Ingress, vLLM, etc.)
│   └── kustomization.yaml
└── overlays/
    └── local/               # Local K3s environment overlay
        ├── kustomization.yaml
        ├── generate-certs.sh
        ├── secret.env.example
        └── certs/           # Generated (gitignored)
```

### Step 1: Generate TLS Certificates

```bash
cd deployment/overlays/local
./generate-certs.sh
```

This generates a self-signed CA and server certificate in `certs/`. You can optionally pass a custom FQDN:
```bash
./generate-certs.sh myapp.example.com
```

### Step 2: Configure LLM Credentials

```bash
cp secret.env.example secret.env
# Edit secret.env with your LLM provider settings
```

For the in-cluster vLLM engine (default):
```
api-base=http://vllm-service.first-responder.svc.cluster.local:8000/v1
model=Qwen/Qwen2.5-1.5B-Instruct
api-key=not-needed
```

### Step 3: Deploy

```bash
kubectl apply -k deployment/overlays/local
```

This creates all resources in the `first-responder` namespace: the app deployment, vLLM engine, services, ingress, TLS secret, and LLM credentials secret.

### Verify

```bash
kubectl get pods -n first-responder
kubectl get ingress -n first-responder
```

## 6. API Testing

Use `curl` to send a log entry to the endpoint:

```bash
export INGRESS_IP=$(kubectl get ingress -n first-responder first-responder-ingress -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

curl -k -X POST https://${INGRESS_IP}/analyze \
   -H "Content-Type: application/json" \
   -d '{"log": "Panic exception error: 500 downstream DB connection timed out"}'
```

##### Expected Output
```json
{
  "summary": "Downstream database connection times out after reaching the deadline",
  "confidence_score": 90,
  "action_items": [
    "Review application logic related to database connections",
    "Check configuration settings for timeouts and resource limits"
  ]
}
```

## 7. Troubleshooting Guide

| Issue | Root Cause | Fix |
| :--- | :--- | :--- |
| Pod Crash (OOM)   | Memory limit exceeded   | Reduce --gpu-memory-utilization to 0.5 or 0.6.
| Illegal Instruction | Missing CPU features  |  Ensure host-passthrough is set in VM configuration.
| Tool Calling 400    | Missing Parser  | Ensure --enable-auto-tool-choice and --tool-call-parser hermes are set.
| Engine Init Fail    | Shared Memory limit | Ensure emptyDir with medium: Memory is mounted to /dev/shm.
