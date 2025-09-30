from .. import pb


def propose(request: pb.ProposeRequest, context) -> pb.ProposeResponse:
    print("Propose called")
    # TODO: Implement the actual propose logic
    return pb.ProposeResponse(skeletons=["skeleton1", "skeleton2"])
