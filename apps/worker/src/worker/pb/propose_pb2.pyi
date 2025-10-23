from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Candidate(_message.Message):
    __slots__ = ("hypothesis", "quantitative", "qualitative")
    HYPOTHESIS_FIELD_NUMBER: _ClassVar[int]
    QUANTITATIVE_FIELD_NUMBER: _ClassVar[int]
    QUALITATIVE_FIELD_NUMBER: _ClassVar[int]
    hypothesis: str
    quantitative: float
    qualitative: str
    def __init__(self, hypothesis: _Optional[str] = ..., quantitative: _Optional[float] = ..., qualitative: _Optional[str] = ...) -> None: ...

class ProposeRequest(_message.Message):
    __slots__ = ("parents", "specification")
    PARENTS_FIELD_NUMBER: _ClassVar[int]
    SPECIFICATION_FIELD_NUMBER: _ClassVar[int]
    parents: _containers.RepeatedCompositeFieldContainer[Candidate]
    specification: _struct_pb2.Struct
    def __init__(self, parents: _Optional[_Iterable[_Union[Candidate, _Mapping]]] = ..., specification: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class ProposeResponse(_message.Message):
    __slots__ = ("hypothesises",)
    HYPOTHESISES_FIELD_NUMBER: _ClassVar[int]
    hypothesises: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, hypothesises: _Optional[_Iterable[str]] = ...) -> None: ...
