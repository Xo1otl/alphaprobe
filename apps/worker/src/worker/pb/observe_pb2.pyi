from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class ObserveRequest(_message.Message):
    __slots__ = ("hypothesis",)
    HYPOTHESIS_FIELD_NUMBER: _ClassVar[int]
    hypothesis: str
    def __init__(self, hypothesis: _Optional[str] = ...) -> None: ...

class ObserveResponse(_message.Message):
    __slots__ = ("hypothesis", "quantitative", "qualitative")
    HYPOTHESIS_FIELD_NUMBER: _ClassVar[int]
    QUANTITATIVE_FIELD_NUMBER: _ClassVar[int]
    QUALITATIVE_FIELD_NUMBER: _ClassVar[int]
    hypothesis: str
    quantitative: float
    qualitative: str
    def __init__(self, hypothesis: _Optional[str] = ..., quantitative: _Optional[float] = ..., qualitative: _Optional[str] = ...) -> None: ...
