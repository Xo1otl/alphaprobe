from concurrent import futures
import grpc
from grpc_reflection.v1alpha import reflection
from . import pb
from . import observe
from . import propose


class LLMSRServicer(pb.LLMSRServicer):
    def Propose(self, request, context):
        return propose.handle(request, context)

    def Observe(self, request, context):
        return observe.handle(request, context)


def main():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    pb.add_LLMSRServicer_to_server(LLMSRServicer(), server)

    SERVICE_NAMES = (
        pb.DESCRIPTOR.services_by_name["LLMSR"].full_name,
        reflection.SERVICE_NAME,
    )
    reflection.enable_server_reflection(SERVICE_NAMES, server)

    server.add_insecure_port("[::]:50051")
    server.start()
    print("llmsr worker gRPC server started on port 50051")
    server.wait_for_termination()


if __name__ == "__main__":
    main()
