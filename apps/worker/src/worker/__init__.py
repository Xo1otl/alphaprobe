import logging
from concurrent import futures

import grpc
from grpc_reflection.v1alpha import reflection

from . import llmsr, pb

logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.INFO)


def main() -> None:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=None))
    servicer = llmsr.new_grpc_servicer()
    pb.add_ProposeServicer_to_server(servicer, server)  # pyright: ignore[reportUnknownMemberType]
    pb.add_ObserveServicer_to_server(servicer, server)  # pyright: ignore[reportUnknownMemberType]

    service_names = (
        pb.PROPOSE_DESCRIPTOR.services_by_name["Propose"].full_name,
        pb.OBSERVE_DESCRIPTOR.services_by_name["Observe"].full_name,
        reflection.SERVICE_NAME,
    )
    reflection.enable_server_reflection(service_names, server)

    server.add_insecure_port("[::]:50051")
    server.start()
    logger.info("alphaprobe worker gRPC server started on port 50051")
    server.wait_for_termination()


if __name__ == "__main__":
    main()
