#include "util.h"
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <string.h>
#include <iostream>
#include <unistd.h>

constexpr size_t RECEIVE_BUF_SIZE = 8;

void processConnection(int sock_fd) {
    std::string input;
    char buf[RECEIVE_BUF_SIZE];

    while (std::cin >> input) {
        std::cout << "input: " << input << std::endl;
        auto ret = write(sock_fd, input.c_str(), input.size());
        assert(ret == input.size());
        ret = read(sock_fd, buf, RECEIVE_BUF_SIZE);
        if (ret == 0) {
            std::cout << "Connection closed...\n";
            return;
        }

        std::string output;
        for (ssize_t i = 0; i < ret; i++) {
            output += buf[i];
        }
        std::cout << "Data from server: " << output << std::endl;
    }
}

void run(int argc, char **argv) {
    assert(argc == 2);
    int sock_fd = socket(AF_INET, SOCK_STREAM, 0);
    assert(sock_fd > 0);

    sockaddr_in server_addr;
    bzero(&server_addr, sizeof(server_addr));
    server_addr.sin_family = AF_INET;
    server_addr.sin_port = htons(SERVER_PORT);
    auto ret = inet_pton(AF_INET, argv[1], &server_addr.sin_addr);
    assert(ret == 1);

    std::cout << "Try to connect...\n";
    ret = connect(sock_fd, reinterpret_cast<sockaddr *>(&server_addr), sizeof(server_addr));
    assert(ret == 0);
    std::cout << "Connection established! start to process...\n";

    processConnection(sock_fd);
}

int main(int argc, char **argv) {
    run(argc, argv);
}