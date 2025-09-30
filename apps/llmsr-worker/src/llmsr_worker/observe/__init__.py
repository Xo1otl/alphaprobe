from .. import pb


def observe(request: pb.ObserveRequest, context) -> pb.ObserveResponse:
    print("Observe called")
    # TODO: Implement the actual observe logic
    return pb.ObserveResponse(skeleton=request.skeleton, score=0.5)
