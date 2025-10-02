from ._types import Request, Response


def handle(request: Request) -> Response:
    if not request.parents:
        raise ValueError("No parents provided")

    best_parent = max(request.parents, key=lambda p: p.score)
    parent_skeleton = best_parent.skeleton
    val = int(parent_skeleton)

    new_skeletons = [
        str(val + 1),
        str(val + 1),
    ]
    return Response(skeletons=new_skeletons)
