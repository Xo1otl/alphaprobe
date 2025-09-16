## Project Overview

-   **Core Idea**: Automated discovery of interpretable models.
-   **Use Cases**: Algorithm/model discovery (physics, math, chemistry), research report generation.
-   **Methodology**: A 'propose-observe' cycle using Go CSP-based pipeline parallelism for I/O-bound stages. A control loop dispatches tasks and propagates results to the state.
-   **Keywords**: Parallel MCTS, Parallel GA, SR, Program Synthesis, Neuro-Symbolic AI, Quality Diversity, Open-Endedness, AlphaGo, Policy Network, Value Network, AlphaFold, Evoformer, Confidence Head, MoE, Test-Time Diffusion, Self-Play Reinforcement Learning, Emergent Abilities, Event Driven Architecutre (EDA).

## Development Conventions

-   **Use Modern Syntax**: The project uses the latest versions of tools and languages (Python 3.13+, Go 1.25+, Bun). Always use the most current and idiomatic syntax and features available. For example, use modern generics syntax in Python (post-3.12 style without `TypeVar` where applicable).
-   **Python `__init__` Methods**: Keep `__init__` methods minimal. Complex initialization logic must be handled by factory functions (like Go).

## Refinement Guideline (推敲)

Apply these guidelines when explicitly asked to "refine" (推敲).

### 1. For Prompts
-   Aggressively apply Occam's Razor. Eliminate redundancy and simplify phrasing to distill the core instruction.

### 2. For Documents
-   Use the following structure as a basis:
    1.  **Introduction:** Describes the overall purpose and background.
    2.  **Abstract:** Provides a high-level summary and cross-cutting perspective.
    3.  **Component Details:** Delve into each component individually in subsequent sections.