# Introduction

To enhance modularity and loose coupling, we will refactor the specialized `Observe` service into a generic, stateless `Evaluate` service. The orchestrator should only be concerned with the discovery lifecycle (the "why"), not the specific mechanics of evaluation (the "how"). This change will create a clearer separation of concerns, making the `worker` a reusable execution engine and the `orchestrator` a pure process manager.

# Overview

The refactoring process involves three main stages: first, we will **Generalize the Interface** to make it domain-agnostic. Next, we will **Update the Worker Implementation** to serve as a generic execution engine. Finally, we will **Update the Orchestrator Client** to use the new, decoupled service.

# Generalize the Interface

The goal is to create a service contract that speaks in terms of generic program execution, not domain-specific hypotheses.

- **How:**
    - Rename `observe.proto` to `evaluate.proto`.
    - Redefine the service and its messages to use generic names (e.g., `service Evaluate`, `message EvaluateRequest`).
    - Replace specific fields like `hypothesis` with a generic `program` field.

# Update the Worker Implementation

The Python worker will become a stateless service that simply executes the program it receives.

- **How:**
    - Implement the new `Evaluate` gRPC servicer.
    - The core logic will be responsible for safely executing the `program` string and returning its quantitative and qualitative results.

# Update the Orchestrator Client

The Go-based orchestrator will adapt to become a client of this new, generic evaluation service.

- **How:**
    - Update the gRPC client to call the `Evaluate` service.
    - The orchestrator will now be responsible for constructing the `program` string to be evaluated and interpreting the generic `EvaluateResponse`.
