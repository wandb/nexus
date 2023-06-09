import json
import socket
import time


SEND_BAD_JSON = False


def main():
    # Set up a socket connection to the server
    server_address = ("localhost", 1337)
    sock = socket.create_connection(server_address)

    try:
        if SEND_BAD_JSON:
            # Create a bad JSON message
            message_data = {"type": 1}
            message_str = json.dumps(message_data) + '\n'

            # Send JSON message to the server
            sock.sendall(message_str.encode())

        # Create a JSON message
        message_data = {"type": "init"}
        # message_data = {"command": "init", "run_id": "idklol"}
        message_str = json.dumps(message_data) + '\n'

        # Send JSON message to the server
        sock.sendall(message_str.encode())

        # Receive and print the response
        while True:
            try:
                response = sock.recv(1024)
                print("Received:", response.decode())
                break
            except socket.timeout:
                time.sleep(1)

    finally:
        # Close the socket connection
        sock.close()


if __name__ == "__main__":
    main()
