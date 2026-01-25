"""Demo of streaming list - Official YouTube Demo"""

import sys
import time
import grpc
import stream_list_pb2
import stream_list_pb2_grpc

def main():
    if len(sys.argv) < 3:
        print("Usage: python3 stream_list_demo.py <OAUTH_TOKEN> <LIVE_CHAT_ID>")
        print("\nOAuth token and live chat ID required")
        sys.exit(1)

    creds = grpc.ssl_channel_credentials()
    with grpc.secure_channel(
        "dns:///youtube.googleapis.com:443", creds
    ) as channel:
        stub = stream_list_pb2_grpc.V3DataLiveChatMessageServiceStub(channel)

        # Using API key authentication (also supported by demo)
        metadata = (("x-goog-api-key", sys.argv[1]),)

        next_page_token = None
        connection_num = 0
        total_responses = 0

        while True:
            connection_num += 1
            connection_start = time.time()

            request = stream_list_pb2.LiveChatMessageListRequest(
                part=["snippet"],
                live_chat_id=sys.argv[2],
                max_results=20,
                page_token=next_page_token,
            )

            print(f"\n{'='*80}")
            print(f"CONNECTION #{connection_num} - Starting stream...")
            print(f"  With pageToken: {next_page_token is not None}")
            print(f"{'='*80}")

            response_count = 0
            try:
                for response in stub.StreamList(request, metadata=metadata):
                    response_count += 1
                    total_responses += 1

                    messages_count = len(response.items)
                    has_next_token = bool(response.next_page_token)

                    print(f"Response #{response_count}: {messages_count} messages, nextToken={has_next_token}")

                    next_page_token = response.next_page_token
                    if not next_page_token:
                        print("  ⚠️  No nextPageToken - breaking")
                        break

            except grpc.RpcError as e:
                print(f"  ❌ gRPC Error: {e.code()} - {e.details()}")
                break
            except Exception as e:
                print(f"  ❌ Exception: {e}")
                break

            connection_duration = time.time() - connection_start
            print(f"\n📊 CONNECTION #{connection_num} ENDED:")
            print(f"  Duration: {connection_duration:.2f} seconds")
            print(f"  Responses received: {response_count}")
            print(f"  Total responses so far: {total_responses}")
            print(f"  Next pageToken: {next_page_token[:20] if next_page_token else 'None'}...")

            # Exit if no next token (stream truly ended)
            if not next_page_token:
                print("\n✅ Stream ended normally (no more data)")
                break

            # Small delay before reconnecting (not in original demo, but let's see)
            # time.sleep(0.1)

if __name__ == "__main__":
    main()
