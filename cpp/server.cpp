#include <iostream>
#include <cstring>
#include <unistd.h>
#include <fcntl.h>
#include <arpa/inet.h>
#include <sys/epoll.h>
#include <sys/socket.h>
#include <map>

constexpr int PORT = 8080;
constexpr int MAX_EVENTS =  1024;
constexpr int BUFFER_SIZE = 1024;

// 设置非阻塞
int setNonBlocking(int fd) {
    int flags = fcntl(fd, F_GETFL, 0);
    if (flags == -1) return -1;
    return fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

class EpollServer {
private:
    int listenFd_;
    int epollFd_;
    std::map<int, std::string> clientBuffers_;  // 每个客户端的读取缓冲

public:
    EpollServer() : listenFd_(-1), epollFd_(-1) {}

    ~EpollServer() {
        if (epollFd_ != -1) close(epollFd_);
        if (listenFd_ != -1) close(listenFd_);
    }

    bool init() {
        // 创建监听 socket
        listenFd_ = socket(AF_INET, SOCK_STREAM, 0);
        if (listenFd_ == -1) {
            perror("socket");
            return false;
        }

        // 端口复用
        int opt = 1;
        setsockopt(listenFd_, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

        // 绑定
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = INADDR_ANY;
        addr.sin_port = htons(PORT);

        if (bind(listenFd_, (sockaddr*)&addr, sizeof(addr)) == -1) {
            perror("bind");
            return false;
        }

        // 监听
        if (listen(listenFd_, SOMAXCONN) == -1) {
            perror("listen");
            return false;
        }

        // 创建 epoll
        epollFd_ = epoll_create1(0);
        if (epollFd_ == -1) {
            perror("epoll_create1");
            return false;
        }

        // 添加监听 socket 到 epoll
        epoll_event ev{};
        ev.events = EPOLLIN;
        ev.data.fd = listenFd_;
        if (epoll_ctl(epollFd_, EPOLL_CTL_ADD, listenFd_, &ev) == -1) {
            perror("epoll_ctl: listenFd");
            return false;
        }

        std::cout << "Server listening on port " << PORT << std::endl;
        return true;
    }

    void run() {
        epoll_event events[MAX_EVENTS];

        while (true) {
            // 等待事件，-1 表示阻塞直到有事件，1000 表示超时 1 秒
            int nfds = epoll_wait(epollFd_, events, MAX_EVENTS, 1000);
            
            if (nfds == -1) {
                perror("epoll_wait");
                break;
            }

            if (nfds == 0) {
                // 超时，可以做心跳检测等
                continue;
            }

            for (int i = 0; i < nfds; ++i) {
                int fd = events[i].data.fd;
                uint32_t ev = events[i].events;

                // 错误处理
                if (ev & (EPOLLERR | EPOLLHUP)) {
                    std::cout << "Client " << fd << " error/hup" << std::endl;
                    closeClient(fd);
                    continue;
                }

                // 新连接
                if (fd == listenFd_) {
                    acceptNewConnection();
                }
                // 客户端数据可读
                else if (ev & EPOLLIN) {
                    handleRead(fd);
                }
                // 客户端可写（用于大流量控制，这里简单处理）
                else if (ev & EPOLLOUT) {
                    // 简单示例：切换回只读模式
                    epoll_event ev{};
                    ev.events = EPOLLIN | EPOLLET;  // ET 模式
                    ev.data.fd = fd;
                    epoll_ctl(epollFd_, EPOLL_CTL_MOD, fd, &ev);
                }
            }
        }
    }

private:
    void acceptNewConnection() {
        sockaddr_in clientAddr{};
        socklen_t len = sizeof(clientAddr);
        
        // 边缘触发模式下需要循环 accept
        while (true) {
            int clientFd = accept(listenFd_, (sockaddr*)&clientAddr, &len);
            if (clientFd == -1) {
                if (errno == EAGAIN || errno == EWOULDBLOCK) {
                    break;  // 没有更多连接了
                }
                perror("accept");
                break;
            }

            // 设置非阻塞
            setNonBlocking(clientFd);

            // 添加客户端到 epoll，使用 ET 模式（边缘触发）
            epoll_event ev{};
            ev.events = EPOLLIN | EPOLLET;  // 读事件 + 边缘触发
            ev.data.fd = clientFd;
            
            if (epoll_ctl(epollFd_, EPOLL_CTL_ADD, clientFd, &ev) == -1) {
                perror("epoll_ctl: clientFd");
                close(clientFd);
                return;
            }

            char ip[INET_ADDRSTRLEN];
            inet_ntop(AF_INET, &clientAddr.sin_addr, ip, sizeof(ip));
            std::cout << "New client " << clientFd 
                      << " from " << ip << ":" << ntohs(clientAddr.sin_port) 
                      << " (total: " << clientBuffers_.size() + 1 << ")" << std::endl;
            
            clientBuffers_[clientFd] = "";  // 初始化缓冲区
        }
    }

    void handleRead(int fd) {
        char buf[BUFFER_SIZE];
        
        // ET 模式必须循环读到 EAGAIN
        while (true) {
            ssize_t n = read(fd, buf, sizeof(buf));
            
            if (n > 0) {
                // 收到数据
                clientBuffers_[fd].append(buf, n);
                
                // 检查是否收到完整消息（简单协议：以 \n 结尾）
                size_t pos;
                while ((pos = clientBuffers_[fd].find('\n')) != std::string::npos) {
                    std::string msg = clientBuffers_[fd].substr(0, pos);
                    clientBuffers_[fd].erase(0, pos + 1);
                    
                    processMessage(fd, msg);
                }
            }
            else if (n == 0) {
                // 客户端关闭
                std::cout << "Client " << fd << " closed connection" << std::endl;
                closeClient(fd);
                return;
            }
            else { // n == -1
                if (errno == EAGAIN || errno == EWOULDBLOCK) {
                    break;  // 数据读完了（ET 模式正常退出）
                }
                else if (errno == EINTR) {
                    continue;  // 被信号中断，重试
                }
                else {
                    perror("read");
                    closeClient(fd);
                    return;
                }
            }
        }
    }

    void processMessage(int fd, const std::string& msg) {
        std::cout << "Client " << fd << " says: " << msg << std::endl;
        
        // 回复客户端
        std::string response = "Echo: " + msg + "\n";
        write(fd, response.c_str(), response.length());
        // 注意：生产环境应该处理 write 的部分发送和 EAGAIN
    }

    void closeClient(int fd) {
        epoll_ctl(epollFd_, EPOLL_CTL_DEL, fd, nullptr);
        close(fd);
        clientBuffers_.erase(fd);
        std::cout << "Client " << fd << " removed" << std::endl;
    }
};

int main() {
    EpollServer server;
    if (!server.init()) {
        return 1;
    }
    server.run();
    return 0;
}