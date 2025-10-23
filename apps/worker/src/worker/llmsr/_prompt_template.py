import ast
import re
import textwrap
from dataclasses import dataclass

from worker import propose


@dataclass(frozen=True)
class Program:
    skeleton: str
    score: float


@dataclass(frozen=True)
class Specification:
    subject: str
    description: str
    max_nparams: int
    num_proposals: int
    header_line: str
    docstring: str


class PromptTemplate(propose.PromptTemplate[Specification, Program]):
    def build(self, specification: Specification, parents: list[Program]) -> tuple[str, propose.ParseFunc]:
        prompt = generate_prompt(specification, parents)
        return prompt, parse_code


def generate_prompt(spec: Specification, parents: list[Program]) -> str:
    context_programs = "\n\n".join(
        f"# Program {i} (Score: {parent.score})\n{textwrap.indent(parent.skeleton, '# ')}"
        for i, parent in enumerate(parents, 1)
    )

    return f"""\
# Subject
{spec.subject}

# Description
{spec.description}

# Context programs
```
{context_programs}
```

# Boilerplate
```
# Imported libraries
import numpy as np
import scipy

# Initialized parameters
MAX_NPARAMS = {spec.max_nparams}
PRAMS_INIT = [0] * MAX_NPARAMS
```

# Proposed program
```
{spec.header_line}
    '''
{textwrap.indent(spec.docstring, "    ")}
    '''
    # TODO: Propose a new equation based on context programs.
```

# Output format
* If you believe physical constants with specific values are needed, use `params` (up to `MAX_NPARAMS`) as placeholder values to be set by the optimizer.
* DO NOT include any comments in the code block because it will be ignored by the parser.

# Task
Suggest {spec.num_proposals} new equations to improve the performance of the function that is inspired by your expert knowledge of the subject.
Please provide ONLY the Python code for the improved function, including the `def` header, enclosed in a code block.
"""  # noqa: E501


def parse_code(text: str) -> list[str]:
    """Extracts all complete Python function definitions from a string."""
    # First, try to extract code from markdown fences if they exist.
    pattern = r"```[^\n]*\n(.*?)```"
    matches = re.findall(pattern, text, re.DOTALL)
    code_to_parse = "\n".join(match.strip() for match in matches)
    if not code_to_parse:
        code_to_parse = text

    functions: list[str] = []
    tree = ast.parse(textwrap.dedent(code_to_parse))
    for node in ast.walk(tree):
        if isinstance(node, ast.FunctionDef):
            # Ensure we only add top-level functions in the current parsing context.
            is_top_level = True
            parent = getattr(node, "parent", None)
            while parent:
                if isinstance(parent, ast.FunctionDef):
                    is_top_level = False
                    break
                parent = getattr(parent, "parent", None)

            if is_top_level:
                functions.append(ast.unparse(node))

    return functions
