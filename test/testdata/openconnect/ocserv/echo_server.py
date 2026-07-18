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


def echo_udp(connection):
    while True:
        data, address = connection.recvfrom(65536)
        connection.sendto(data, address)


udp_listener = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
udp_listener.bind(("0.0.0.0", 18080))
threading.Thread(target=echo_udp, args=(udp_listener,), daemon=True).start()

tcp_listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
tcp_listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
tcp_listener.bind(("0.0.0.0", 18080))
tcp_listener.listen()
print("openconnect echo ready", flush=True)
while True:
    accepted, _ = tcp_listener.accept()
    threading.Thread(target=echo, args=(accepted,), daemon=True).start()
