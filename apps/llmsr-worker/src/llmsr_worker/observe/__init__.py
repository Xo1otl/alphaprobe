import grpc
from .. import pb


def handle(request: pb.ObserveRequest, context) -> pb.ObserveResponse:
    skeleton = request.skeleton
    try:
        val = int(skeleton)
    except ValueError:
        context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
        context.set_details(f"Invalid skeleton format: '{skeleton}' is not an integer.")
        return pb.ObserveResponse()

    score = float(val)
    return pb.ObserveResponse(skeleton=skeleton, score=score)
