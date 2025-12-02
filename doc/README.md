# kubectl-medic

A **minimal, fast, TUI-based kubectl plugin** that helps users troubleshoot **pods** and **namespaces** with clarity and simplicity.  
`kubectl-medic` focuses on **diagnostics, visibility, and actionable suggestions** — not modifying or managing cluster resources.

Built with **Go**, **Bubble Tea**, **Cobra**, and **client-go**.

---

## ✨ Purpose

`kubectl-medic` exists to help:

- **Developers** who are not Kubernetes experts  
- **On-call engineers** who need rapid triage  
- **SRE/Platform teams** who want a consistent tool to guide others  

It makes troubleshooting intuitive by presenting:

- Namespace list  
- Pod list  
- Pod details & events  
- A guided diagnostic summary  
- Logs viewer  
- Safe suggestions + recommended `kubectl` commands  

All accessible with **2–3 key presses**, using a clean, unified TUI.

---

## 🎯 Goals

- Provide a **simple guided workflow**:  
  **Namespace → Pod → Diagnose → Suggested actions**
- Highlight issues that are **not obvious** at first glance.
- Enable users to **understand problems**, not just run commands.
- Make troubleshooting faster without replacing kubectl.
- Maintain a **safe, read-only-first** philosophy.

---

## 🚫 Non-Goals (v1)

- No automated remediation  
- No edits, patches, or destructive actions  
- No advanced filtering or search across the whole cluster  
- No logs streaming across multiple pods  
- No node-level or cluster-level diagnostics  
- No heavy abstractions beyond pods & namespaces  

---

## 👤 Target Users

### 1. Junior Developer / On-Call Engineer
Needs quick clarity:
- “Why is my pod not running?”
- “What does this status mean?”
- “What should I check next?”

### 2. SRE / Platform Engineer
Wants:
- Consistent triage workflow  
- Fast visibility  
- A tool to guide non-experts  

---

## 🔧 Core Features (v1)

### ✔ Namespace Browser
- List namespaces (with filter + sort)
- See status, age
- Quickly detect problematic namespaces
- Hotkey: `Enter` loads pods

### ✔ Pod Browser
- Name  
- Ready containers  
- Status (colored)  
- Restarts  
- Age  
- Sort & filter  
- Problem pods highlighted  

### ✔ Pod Details
- Metadata  
- Container states  
- Readiness/Liveness presence  
- Conditions (with reasons)  
- Relevant events  
- Describe-style summary view  

### ✔ Diagnostics Engine
Runs a structured set of checks:

- CrashLoopBackOff analysis  
- Image pull errors  
- Scheduling issues  
- Resource quota & limits  
- Node pressure scheduling rejections  
- Container restart reasons  
- Probe misconfiguration indicators  

Outputs a **human-readable diagnosis**, e.g.:

- “Pod stuck in *CrashLoopBackOff* due to exit code 1”
- “ImagePullBackOff — check image name or registry credentials”
- “Insufficient CPU for scheduling on available nodes”

### ✔ Logs Viewer
- Scrollable  
- Switch containers  
- Optional follow mode  
- Simple, clean presentation  

### ✔ Safe Actions (No Mutation)
`kubectl-medic` **does not modify resources**.  
Instead, it provides **suggested commands**, e.g.:
kubectl describe pod myapp-123 -n staging
kubectl logs myapp-123 -n staging --container api


---

## 🎨 UI/UX Design Principles

- Smooth navigation (mostly arrow keys + a few hotkeys)
- 2–3 key presses for every major task  
- Clean 3-pane layout:
  - **Left**: Namespaces  
  - **Middle**: Pods  
  - **Right**: Details / Diagnostics / Logs  
- Consistent colors for:
  - Success
  - Warning
  - Error
- One central theme file (Lip Gloss)

---

## 🏗 Tech Stack

- **Go**
- **Cobra** → CLI plugin wrapper (`kubectl medic`)
- **Bubble Tea** → TUI
- **Lip Gloss** → Styling
- **client-go** → Kubernetes integration

---

## 📂 Suggested Project Structure

^^^

kubectl-medic/
│
├── cmd/
│ └── medic/ # Cobra root command
│
├── internal/
│ ├── tui/ # Bubble Tea models & views
│ │ ├── namespace/
│ │ ├── pods/
│ │ ├── details/
│ │ ├── logs/
│ │ └── diagnostics/
│ │
│ ├── kube/ # client-go wrappers
│ ├── analysis/ # diagnostics engine
│ ├── state/ # shared app state
│ └── theme/ # colors & UI components
│
├── docs/ # spec documents (use cases, requirements)
│
└── README.md

^^^


---

## 📈 Roadmap (Post-v1 Ideas)

- Export diagnostics report to file  
- Context switching  
- Node-level analysis  
- Support for Deployments, Jobs, StatefulSets  
- Ephemeral debug container support  

---

## 🧪 Status

`kubectl-medic` is currently in **pre-development spec stage**.  
The next steps are:

1. Finalize architecture  
2. Build TUI prototype  
3. Integrate Kubernetes client  
4. Implement diagnostics engine  

---

## 🤝 Philosophy

**kubectl-medic will never modify your cluster.**  
It will guide, teach, and surface insights — but **you always stay in control**.

---

## 📜 License

MIT (or choose your own)

---

## ❤️ Contributions

Feedback, feature ideas, and PRs are welcome once development begins.
