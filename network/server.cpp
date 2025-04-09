#include "util.h"
#include <cassert>
#include <cerrno>
#include <netinet/in.h>
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>
#include <vector>
#include <iostream>
#include <string>
#include <stdio.h>

constexpr uint32_t BUF_SIZE = 8;

void processReceivedBuf(int fd, char *buf, ssize_t n) {
    std::string s;
    s.reserve(n);
    std::cout << "n: " << n << std::endl;
    for (ssize_t i = 0; i < n; i++) {
        s = s + buf[i];
    }
    std::cout << "processReceivedBuf output: " << s << std::endl;
    auto ret = write(fd, buf, n);
    assert(ret == n);
}

void processConnection(int sock_fd) {
    // std::vector<char> conn_data;
    // conn_data.reserve(4096);
    
    char buf[BUF_SIZE];
    ssize_t n;

    while (true) {
        while ((n = read(sock_fd, buf, BUF_SIZE)) > 0)
            processReceivedBuf(sock_fd, buf, n);
        
        // When we are interrupted, then n is lower than 0 and errno == INTR
        if (n < 0 && errno == EINTR)
            continue;
        else if (n < 0)
            assert(false);
    }
}

void run() {
    int listen_fd = socket(AF_INET, SOCK_STREAM, 0);
    assert(listen_fd > 0);

    sockaddr_in server_addr;
    bzero(&server_addr, sizeof(server_addr));

    server_addr.sin_family = AF_INET;
    server_addr.sin_addr.s_addr = htonl(INADDR_ANY);
    server_addr.sin_port = htons(SERVER_PORT);
    auto ret = bind(listen_fd, reinterpret_cast<sockaddr *>(&server_addr), sizeof(server_addr));
    if (ret != 0) {
        std::cout << "errno: " << errno << std::endl;
        assert(false);
    }

    std::cout << "Start to listen...\n";
    ret = listen(listen_fd, 3000);
    assert(ret == 0);

    sockaddr_in child_addr;
    while (true) {
        socklen_t child_len = sizeof(child_addr);
        auto connect_fd = accept(listen_fd, reinterpret_cast<sockaddr *>(&child_addr), &child_len);
        if (connect_fd < 0) {
            std::cout << "errno: " << errno << std::endl;
            assert(false);
        }

        std::cout << "Receive one connection\n";
        
        __pid_t child_pid = fork();
        if (child_pid == 0) {
            // We are child process
            // As child does not need to listen new connection, we need to close it
            ret = close(listen_fd);
            assert(ret == 0);

            processConnection(connect_fd);
            exit(0);
        }

        // As server does not need connect_fd, we should close it
        ret = close(connect_fd);
        assert(ret == 0);
    }
}

int main() {
    run();
}
