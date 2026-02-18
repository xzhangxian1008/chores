#include <iostream>
#include <cstring>
#include <unistd.h>
#include <arpa/inet.h>
#include <sys/socket.h>
#include <thread>
#include <string>

constexpr std::string_view SERVER_IP = "127.0.0.1";
constexpr int PORT = 8765;
constexpr int BUFFER_SIZE = 1024;

class TcpClient {
private:
    int sockFd_;

public:
    TcpClient() : sockFd_(-1) {}

    ~TcpClient() {
        if (sockFd_ != -1) close(sockFd_);
    }

    bool connectToServer() {
        sockFd_ = socket(AF_INET, SOCK_STREAM, 0);
        if (sockFd_ == -1) {
            perror("socket");
            return false;
        }

        sockaddr_in serverAddr{};
        serverAddr.sin_family = AF_INET;
        serverAddr.sin_port = htons(PORT);
        inet_pton(AF_INET, SERVER_IP.data(), &serverAddr.sin_addr);

        if (connect(sockFd_, (sockaddr*)&serverAddr, sizeof(serverAddr)) == -1) {
            perror("connect");
            return false;
        }

        std::cout << "Connected to server " << SERVER_IP << ":" << PORT << std::endl;
        return true;
    }

    void run() {
        std::thread recvThread(&TcpClient::receiveLoop, this);
        
        std::string line;
        while (std::getline(std::cin, line)) {
            line += "\n";  // 添加换行作为消息分隔符
            std::cout << "Send: " << line << std::endl;
            ssize_t sent = send(sockFd_, line.c_str(), line.length(), 0);
            if (sent == -1) {
                perror("send");
                break;
            }
        }

        close(sockFd_);
        sockFd_ = -1;
        recvThread.join();
    }

private:
    void receiveLoop() {
        char buf[BUFFER_SIZE];
        while (sockFd_ != -1) {
            ssize_t n = recv(sockFd_, buf, sizeof(buf) - 1, 0);
            if (n > 0) {
                buf[n] = '\0';
                std::cout << "[Server] " << buf;
            }
            else if (n == 0) {
                std::cout << "Server closed connection" << std::endl;
                break;
            }
            else {
                if (errno != EINTR) {
                    perror("recv");
                    break;
                }
            }
        }
    }
};

int main() {
    TcpClient client;
    if (!client.connectToServer()) {
        return 1;
    }
    client.run();
    return 0;
}