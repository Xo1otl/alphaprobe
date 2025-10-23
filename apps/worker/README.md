# Introduction

The `orchestrator` (Go) manages the search process, while computationally intensive tasks are delegated to Python. The `worker` is a gRPC service that provides a type-safe interface between these two environments.

# Overview

`worker` is a Python-based gRPC server. It exposes multiple **`Propose` Services** for generating candidate hypotheses and an **`Observe` Service** for evaluating them. This document also covers **Usage**.

# Propose Services

The `Propose` services are a set of RPCs used by the `orchestrator` to generate new candidate hypotheses. Each service corresponds to a specific proposal strategy (e.g., `LLMSRPropose`, `CodePropose`) and has its own strongly-typed request message.

This design uses the `.proto` schema to define the required configuration for each strategy. To add a new proposal method, a new RPC and its corresponding request/response messages must be added to the `.proto` file, ensuring type safety between the client and server.

# Observe Service

The `Observe` service is an RPC called by the `orchestrator` to evaluate the performance of a candidate hypothesis. It receives a hypothesis, optimizes its free parameters against a dataset, and returns quantitative and qualitative performance scores.

# Usage

To start the gRPC server, run the following command from the project's root directory:

```sh
uv run worker
```