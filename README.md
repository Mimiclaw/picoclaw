# Mimiclaw

> Build Your AI Workforce.
> Orchestrate Autonomous Employees.
> Turn OpenClaw into a Self-Operating Company.

<img width="1536" height="459" alt="banner-sm" src="https://github.com/user-attachments/assets/ce7441ba-559f-441f-837b-896e08926b96" />

[中文](README.zh.md) | **English**

---

## What is Mimiclaw?

Mimiclaw is the **Workforce Layer** of OpenClaw.

It is not another agent.
It is not a wrapper.
It is not a multi-agent demo.

Mimiclaw is the system that allows OpenClaw to:

* Create employees
* Assign responsibilities
* Parallelize execution
* Supervise progress
* Enforce approval boundaries
* Control digital labor at scale

It turns intelligence into organization.

---

## The Idea

Every intelligent system eventually faces the same limitation:

> A single agent cannot scale like a company.

Mimiclaw solves this.

It enables OpenClaw to:

* Spawn independent AI employees
* Assign them structured roles
* Allow them to communicate
* Execute tasks in parallel
* Report progress in stages
* Escalate asset-sensitive operations for approval
* Be destroyed, replaced, or reconfigured at will

This is not agent chaining.
This is structured digital labor.

---

## Conceptual Model

OpenClaw becomes the CEO.

Employees become:

* Researchers
* Coders
* Operators
* Analysts
* Executors

All autonomous.
All isolated.
All supervised.

```text
                ┌────────────────────┐
                │      OpenClaw      │
                │  (Supervisor Core) │
                └─────────┬──────────┘
                          │
             ┌────────────┼────────────┐
             │            │            │
        ┌────▼────┐  ┌────▼────┐  ┌────▼────┐
        │Employee 1│  │Employee 2│  │Employee 3│
        │(mimiclaw)│  │(mimiclaw)│  │(mimiclaw)│
        └──────────┘  └──────────┘  └──────────┘
```

OpenClaw allocates vision.
Mimiclaw builds execution.

---

## Architecture Philosophy

Mimiclaw is built around five principles:

### 1. Isolation

Each employee is:

* An independent mimiclaw process
* Running in its own workspace
* Equipped with its own:

  * Skills
  * Tools
  * API keys
  * Wallet abstraction
  * Memory
* Fully destroyable

Employees cannot spawn other employees.

Hierarchy remains controlled.

---

### 2. Parallelism

Employees execute concurrently.

* No preemption.
* Priority equals sorting.
* User defines maximum concurrent employees.
* When limit is reached:

  * OpenClaw may destroy or replace agents.

This models resource allocation inside a company.

---

### 3. Structured Communication

All communication happens locally via WebSocket.

* OpenClaw ↔ Employees
* Employee ↔ Employee

Messages are JSON-based.

Employees:

* Receive structured tasks
* Send progress checkpoints
* Report final results
* Request asset approvals

OpenClaw logs everything.

This creates a transparent communication graph.

---

### 4. Approval-Based Asset Security

Employees may propose asset actions.

They may never sign.

Workflow:

1. Employee proposes asset operation
2. OpenClaw asks user
3. User approves
4. OpenClaw signs
5. Result returned to employee

All signatures are centralized.

Authority is never delegated.

---

### 5. Supervisor Control

OpenClaw is:

* Task allocator
* Signature authority
* Health monitor
* Lifecycle controller
* WebSocket hub

Employees are workers.

OpenClaw is control.

---

## Health Model

An employee is considered alive only if:

1. Process exists
2. WebSocket heartbeat is active
3. Agent self-report is valid

Failure in any dimension triggers supervision logic.

---

## JSON Protocol (Workforce Language)

### Task Schema

```json
{
  "task_id": "uuid",
  "from": "openclaw",
  "to": "employee-1",
  "priority": 5,
  "goal": "Analyze smart contract risk",
  "inputs": {},
  "tools_allowed": ["bash", "curl"],
  "assets_flag": false,
  "approval_required": false,
  "timestamp": 1700000000
}
```

---

### Progress Schema

```json
{
  "task_id": "uuid",
  "status": "in_progress",
  "progress": 0.45,
  "message": "Parsing contract ABI...",
  "timestamp": 1700000100
}
```

---

### Asset Request Schema

```json
{
  "task_id": "uuid",
  "type": "asset_request",
  "action": "transfer",
  "payload": {
    "to": "0x...",
    "amount": "1.5 ETH"
  }
}
```

---

### Result Schema

```json
{
  "task_id": "uuid",
  "status": "completed",
  "result": {},
  "timestamp": 1700000200
}
```

---

## Dashboard Vision

Mimiclaw includes a workforce dashboard.

It visualizes:

* Employee topology
* Task allocation
* Priority levels
* Progress checkpoints
* Communication logs
* Supervisor approval events

Topology Example:

```text
[OpenClaw]
    │
    ├── [Researcher-1]
    ├── [Coder-1]
    └── [Operator-1]
```

Full message tracing is preserved.

This is not only execution.

It is observability.

---

## Multi-Model Intelligence

OpenClaw assigns models per employee or per task.

* Model switching is centralized.
* Employees do not choose models.
* Strategy remains in supervisor.

This prevents chaos.

---

## Lifecycle Commands

Mimiclaw exposes full workforce control:

* create employee
* delete employee
* assign task
* broadcast task
* list employees
* get status
* fetch logs

The entire company is programmable.

---

## Designed For

* Autonomous AI companies
* DAO operations
* Parallel research teams
* Code generation pipelines
* Automated trading assistants
* Continuous security auditing

---

## What Mimiclaw Is Not

* Not a token system
* Not a governance layer
* Not a resource quota manager
* Not recursive agent spawning
* Not cost-optimized cloud infrastructure

It is an execution architecture.

---

## The Bigger Vision

Mimiclaw represents a shift:

From:
Single intelligent system

To:
Structured autonomous workforce

From:
Tool calling

To:
Organized digital labor

From:
AI assistant

To:
AI company

---

## Worker WebSocket Configuration

Mimiclaw supports a dedicated worker WebSocket channel defined by
[`worker_ws.md`](./worker_ws.md).

Configure it in `config.json`:

```json
{
  "channels": {
    "worker_ws": {
      "enabled": true,
      "address": "ws://127.0.0.1:3001/ws",
      "role": "employee",
      "name": "Researcher-1",
      "tags": ["research", "risk"],
      "authkey": "",
      "identity_file": "~/.mimiclaw/worker_ws_identity.json",
      "reconnect_interval": 5,
      "ping_interval": 15,
      "allow_from": []
    }
  }
}
```

`address`, `role`, `name`, and `tags` are configurable from `config.json`.

---

## Status

Design complete.
Protocol defined.
WebSocket architecture in progress.
Process supervision documentation pending.

---

## License

MIT
