from collections.abc import Callable
from dataclasses import dataclass


@dataclass(frozen=True)
class Request[S, P]:
    parents: list[P]
    specification: S


@dataclass(frozen=True)
class Response:
    hypothesises: list[str]


type HandlerFunc[S, P] = Callable[[Request[S, P]], Response]
