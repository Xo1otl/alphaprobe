import ast
from dataclasses import dataclass
from typing import Any

import numpy as np

from worker import llmsr, propose


@dataclass
class WrappedLLM:
    llm: llmsr.LLM

    def generate(self, prompt: str) -> str:
        answer = self.llm.generate(prompt)
        print(answer)
        return answer


def test_propose() -> None:
    handle = propose.new_handler(llmsr.PromptTemplate(), WrappedLLM(llmsr.LLM()))
    specification = llmsr.Specification(
        subject="stress strain model",
        description="Find the mathematical function skeleton that represents stress, given data on strain and temperature in an Aluminium rod for both elastic and plastic regions.",
        max_nparams=3,
        num_proposals=2,
        header_line="def equation(strain: np.ndarray, temp: np.ndarray, params: np.ndarray) -> np.ndarray:",
        docstring="""\
Mathematical function for stress in Aluminium rod

Args:
    strain: A numpy array representing observations of strain.
    temp: A numpy array representing observations of temperature.
    params: Array of numeric constants or parameters to be optimized

Return:
    A numpy array representing stress as the result of applying the mathematical function to the inputs.
""",
    )
    parents = [
        llmsr.Program(
            skeleton="""\
def equation(strain: np.ndarray, temp: np.ndarray, params: np.ndarray) -> np.ndarray:
    stress = params[0] * strain + params[1] * temp
    return stress
""",
            score=0.0,
        ),
    ]
    response = handle(propose.Request(parents=parents, specification=specification))
    print(response.hypothesises)
    tree = ast.parse(response.hypothesises[0])
    code_obj = compile(tree, filename="<string>", mode="exec")
    ns: dict[str, Any] = {}
    ns["np"] = np
    exec(code_obj, ns)
