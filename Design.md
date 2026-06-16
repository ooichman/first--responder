This assignment is designed to evaluate your practical application skills, specifically your proficiency in modern development tooling, application deployment, and the integration of AI/Agent concepts. 
We encourage the use of any AI tools, such as Large Language Models (LLMs) or coding assistants, to aid in the completion of this task. 
The Task: The Intelligent Triage Agent 
Build a containerized tool that acts as a "First Responder" for system errors. 
1. Input: The system should accept a raw error log (via an API endpoint or UI text box). For example: {500/403: “<Description/Error message>”} 
2. Agent Logic: The agent must use an LLM to "reason" about the error. It must be equipped with at least one Tool (a mock function that retrieves "Company Troubleshooting Docs") to help it decide on a solution. 
3. Output: The tool should return a structured JSON response containing: ○ A 1-sentence summary of the problem. 
○ A "Confidence Score" (0–100%). 
○ Recommended "Action Items" (e.g., "Restart Pod," "Check DB Credentials," or "Escalate to Senior Dev"). 
4. Infrastructure: The app must run as a Pod in Kubernetes. Use any LLM of your choice, can be any cloud model or also be locally in the cluster (ollama, etc) 
Notes: 
1. The raw error logs input can be a mock/fabricated and does not require real app to stream it 
2. The LLM decision and actions based on the error logs can be custom/arbitrary and what is considered reasonable. For example, if the raw error is 403, an action item of “restart pod” is not likely. 
Continue reading the next page
Ideal task: 
Requirement 
Description
Project Functionality 
The application or tool is fully functional and demonstrably meet the specific goals you define in your accompanying documentation.
Containerization & Deployment 
The project is containerized. 
Deployable on a Kubernetes cluster. You may use a local cluster (e.g., minikube, k3s) or a cloud provider's free tier. Make sure to also include all necessary Kubernetes manifest YAML files (or setup instructions).
Documentation 
Clear, comprehensive instructions must be provided detailing how to build, deploy, and run your complete project environment.
Code Quality 
The submitted code must be well-organized, readable, maintainable, and adhere to standard best practices.







Project Presentation: 
Following the submission deadline, candidates will be invited to a live, virtual session. During this session, you will: 
1. Present your project and provide a brief live demonstration. 
2. Answer technical questions about your design, implementation choices, and the deployment architecture. 
Good luck!
