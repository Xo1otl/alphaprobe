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
-   **Structure:** Adhere to the following three-part structure. Use explicit Japanese titles for sections and do not use numbering.
    1.  **導入:** Start with the concrete problem, specific event, or motivation that initiated the project.
    2.  **概要:** Describe the project's generalized purpose, overall goals, and any cross-cutting concepts (e.g., architectural principles).
    3.  **コンポーネント詳細:** Detail individual components, models, or concepts. Present these as a flat list, using the same heading level for each. Avoid deep nesting.
-   **Tone:** Write in a formal, declarative style (「～だ」「～である」調).