import grpc
from .. import pb


def handle(request: pb.ProposeRequest, context) -> pb.ProposeResponse:
    if not request.parents:
        context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
        context.set_details("No parents provided")
        return pb.ProposeResponse()

    best_parent = max(request.parents, key=lambda p: p.score)
    parent_skeleton = best_parent.skeleton
    try:
        val = int(parent_skeleton)
    except ValueError:
        context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
        context.set_details(
            f"Invalid skeleton format: '{parent_skeleton}' is not an integer."
        )
        return pb.ProposeResponse()

    new_skeletons = [
        str(val + 1),
        str(val + 1),
    ]
    return pb.ProposeResponse(skeletons=new_skeletons)
