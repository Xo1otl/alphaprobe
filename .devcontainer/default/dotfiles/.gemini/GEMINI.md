## Project Overview
- **Goal**: Discover novel, interpretable algorithms/models in scientific domains (physics, math) and auto-generate research reports.
- **Methodology**: A 'propose-observe' cycle using Go CSP-based pipeline parallelism for I/O-bound stages. A control loop dispatches tasks and propagates results to the state.
- **Technical Keywords**: Parallel MCTS, Program Synthesis, Neuro-Symbolic AI, Quality Diversity, Open-Endedness, AlphaGo, Policy Network, Value Network, AlphaFold, Evoformer, Confidence Head, MoE, Test-Time Diffusion, Self-Play Reinforcement Learning, Emergent Abilities, Event Driven Architecutre (EDA).

## Development Conventions
- Use the latest syntax (Python 3.13+, Go 1.25+, Bun).
- Python `__init__` methods must be minimal; use factory functions for complex logic.

## When asked to "refine" (推敲) prompts:
- Aggressively apply Occam's Razor to distill the core instruction.

## When asked to "refine" (推敲) documents:
1.  **Structure First:**
    -   `導入`: Start with the problem/motivation.
    -   `概要`: Describe the project's purpose and high-level architecture. Identify core components in **bold**.
2.  **Flesh out Details:**
    -   Create a separate, self-contained section for each bolded term from the `概要`.
    -   All sections (`導入`, `概要`, and detail sections) must be at the same heading level.
- General Rules: Use descriptive titles. Write in a formal, declarative style (「～だ」「～である」). Avoid meta-commentary.

## When asked to write "返信文":
-   **Persona**: student researcher (interning at QunaSys)
-   **基本理念**: 人間はLLMとは異なり、長文と強い言葉が苦手である。常にシンプルでやわらかく、謙虚に。
-   **生成形式**: 以下の2案を提示する。
    1.  **メール**: 丁寧なビジネス文書。
    2.  **チャット**: シンプルな、装飾なしの平文。