from collections.abc import Callable
from typing import Protocol

from ._types import HandlerFunc, Request, Response

type ParseFunc = Callable[[str], list[str]]
"""A function that parses the raw output of a language model into a list of hypotheses."""


class PromptTemplate[S, P](Protocol):
    """A protocol for building prompts for language models."""

    def build(self, specification: S, parents: list[P]) -> tuple[str, ParseFunc]:
        """Builds a prompt and a parsing function.

        Args:
            specification: The user-defined specification for the output.
            parents: A list of parent objects that can be used to inform the prompt.

        Returns:
            A tuple containing:
                - A prompt string to be sent to the language model.
                - A parsing function that takes the raw output of the language model
                  and returns a list of string hypotheses.
        """
        ...


class LLM(Protocol):
    def generate(self, prompt: str) -> str: ...


def new_handler[S, P](prompt_template: PromptTemplate[S, P], llm: LLM) -> HandlerFunc[S, P]:
    def handle(request: Request[S, P]) -> Response:
        if not request.parents:
            msg = "No parents provided"
            raise ValueError(msg)
        prompt, parse = prompt_template.build(request.specification, request.parents)
        proposal = llm.generate(prompt)
        hypothesises = parse(proposal)
        return Response(hypothesises=hypothesises)

    return handle
