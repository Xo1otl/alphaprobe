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
-   **Generation Process:** Refine documents by following a generative, two-step process.
    1.  **First, establish the structure:**
        *   Write the **`導入`** section, starting with the concrete problem or motivation.
        *   Write the **`概要`** section. This section's role is to describe the project's overall purpose and explain the high-level architecture or the relationships between the core functional parts.
        *   When introducing these core parts for the first time in the `概要`, **identify them using bold text (`**`)**. Do not go into their specific details here. This effectively creates the document's skeleton.
    2.  **Then, flesh out the details:**
        *   For each term that was bolded in the `概要`, create a dedicated section with the bolded term as its heading.
        *   These new sections must be at the **same heading level** as `導入` and `概要`, creating a flat structure.
        *   The content within these detail sections should be self-contained.
-   **General Rules:**
    *   Use explicit, descriptive titles for sections.
    *   Write in a formal, declarative style (「～だ」「～である」調).
    *   Avoid meta-commentary that describes the document's own structure.