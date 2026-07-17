#!/usr/bin/env python3
import socket
import threading


def echo(connection):
    with connection:
        while True:
            data = connection.recv(65536)
            if not data:
                return
            connection.sendall(data)


listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
listener.bind(("0.0.0.0", 18080))
listener.listen()
print("openconnect echo ready", flush=True)
while True:
    accepted, _ = listener.accept()
    threading.Thread(target=echo, args=(accepted,), daemon=True).start()
