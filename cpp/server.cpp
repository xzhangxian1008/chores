#include <arpa/inet.h>
#include <csignal>
#include <cstring>
#include <iostream>
#include <mutex>
#include <string>
#include <thread>
#include <unordered_set>
#include <unistd.h>
#include <vector>
#include <sys/socket.h>
#include <rocksdb/db.h>

#include "util.h"

const std::string ROCKSDB_PAth = "/DATA/disk3/xzx/chores/cpp/server";

constexpr std::string_view GET_OP = "get";
constexpr std::string_view PUT_OP = "put";
constexpr std::string_view DELETE_OP = "delete";

constexpr int PORT = 8765;
constexpr int BACK_LOG = 128;
constexpr size_t BUFFER_SIZE = 4096;
volatile std::sig_atomic_t g_stop = 0;
volatile std::sig_atomic_t g_listen_fd = -1;
std::mutex g_clients_mutex;
std::unordered_set<int> g_client_fds;

rocksdb::DB * db;

struct Status {
    enum Code {
        Ok,
        Error
    };

    Status() = default;

    void setCode(Code code) { code_ = code; }
    void setMsg(const String & msg) { msg_ = msg; }
    void setErrMsg(const String & msg) { error_msg_ = msg; }

    bool isOk() const { return code_ == Code::Ok; }
    std::string getMsg() const { return msg_; }
    std::string getErrMsg() const { return error_msg_; }

    Code code_;
    std::string msg_;
    std::string error_msg_;
};

void parseMessage(
    const std::string & msg,
    std::string & op,
    std::string & key,
    std::string & val
) {
    char delimiter = ' ';
    std::vector<std::string> tokens;
    size_t start = 0;
    size_t end = msg.find(delimiter);
    
    while (end != std::string::npos) {
        tokens.push_back(msg.substr(start, end - start));
        start = end + 1;
        end = msg.find(delimiter, start);
    }
    tokens.push_back(msg.substr(start));
    
    if (tokens.size() == 0) {
        std::cout << "Nothing is in the request\n";
    } else if (tokens.size() > 3) {
        std::cout << "Warning: tokens.size() > 3\n";
    } else {
        auto token_count = tokens.size();
        op = tokens[0];
        if (token_count > 1)
            key = tokens[1];
        if (token_count > 2)
            val = tokens[2];
    }
}

struct ClientWorkSpace {
    Status processMessage(const std::string & msg) {
        parseMessage(msg, op_, key_, val_);
        
        Status ret_status;
        if (op_ == GET_OP) {
            auto op_status = db->Get(rocksdb::ReadOptions(), key_, &val_);
            if (op_status.ok()) {
                ret_status.setCode(Status::Code::Ok);
                ret_status.setMsg("Get success. <key: " + key_ + ", val: " + val_ + ">");
            } else {
                ret_status.setCode(Status::Code::Error);
                ret_status.setErrMsg("Fail to execute 'get' op for key " + key_);
            }
        } else if (op_ == PUT_OP) {
            auto op_status = db->Put(rocksdb::WriteOptions(), key_, val_);
            if (op_status.ok()) {
                ret_status.setCode(Status::Code::Ok);
                ret_status.setMsg("Put success. <key: " + key_ + ", val: " + val_ + ">");
            } else {
                ret_status.setCode(Status::Code::Error);
                ret_status.setErrMsg("Fail to execute 'put' op for <key: " + key_ + "," + " val: " + val_ + ">");
            }
        } else if (op_ == DELETE_OP) {
            auto op_status =  db->Delete(rocksdb::WriteOptions(), key_);
            if (op_status.ok()) {
                ret_status.setCode(Status::Code::Ok);
                ret_status.setMsg("Delete success. key: " + key_);
            } else {
                ret_status.setCode(Status::Code::Error);
                ret_status.setErrMsg("Fail to execute 'delete' op for key " + key_);
            }
        } else {
            ret_status.setCode(Status::Code::Error);
            ret_status.setErrMsg("Warning: unknown operation " + op_);
        }

        return ret_status;
    }
    
    std::string buffer_;
    std::string op_;
    std::string key_;
    std::string val_;
};

class ClientManager {
public:
    size_t getClientCount() {
        std::lock_guard<std::mutex> lock(mu_);
        return clients_.size();
    }

    void addClient(int client_fd) {
        std::lock_guard<std::mutex> lock(mu_);
        clients_.insert({client_fd, ClientWorkSpace()});
    }

    void deleteClient(int client_fd) {
        std::lock_guard<std::mutex> lock(mu_);
        clients_.erase(client_fd);
    }

    ClientWorkSpace * getClient(int client_fd) {
        std::lock_guard<std::mutex> lock(mu_);
        auto iter = clients_.find(client_fd);
        if (iter == clients_.end())
            throw std::exception();
        return &(iter->second);
    }
private:
    std::unordered_map<int, ClientWorkSpace> clients_;
    std::mutex mu_;
};

ClientManager client_manager;

void handleSignal(int) {
    g_stop = 1;
    if (g_listen_fd >= 0) {
        // close is async-signal-safe and can unblock accept() immediately.
        close(static_cast<int>(g_listen_fd));
        g_listen_fd = -1;
    }
}

void registerClientFd(int client_fd) {
    std::lock_guard<std::mutex> lock(g_clients_mutex);
    g_client_fds.insert(client_fd);
}

void unregisterClientFd(int client_fd) {
    std::lock_guard<std::mutex> lock(g_clients_mutex);
    g_client_fds.erase(client_fd);
    client_manager.deleteClient(client_fd);
}

std::vector<int> snapshotClientFds() {
    std::lock_guard<std::mutex> lock(g_clients_mutex);
    return std::vector<int>(g_client_fds.begin(), g_client_fds.end());
}

void handleClient(int client_fd, sockaddr_in client_addr) {
    char ip[INET_ADDRSTRLEN] = {0};
    inet_ntop(AF_INET, &client_addr.sin_addr, ip, sizeof(ip));
    int client_port = ntohs(client_addr.sin_port);
    std::cout << "Client connected: " << ip << ":" << client_port << std::endl;

    client_manager.addClient(client_fd);
    auto * client = client_manager.getClient(client_fd);

    std::vector<char> recv_buf(BUFFER_SIZE);
    std::string pending_buffer;
    while (!g_stop) {
        ssize_t n = recv(client_fd, recv_buf.data(), recv_buf.size(), 0);
        if (n > 0) {
            pending_buffer.append(recv_buf.data(), static_cast<size_t>(n));

            // 按行协议处理：只有读到 '\n' 才算一条完整消息。
            size_t pos = 0;
            while ((pos = pending_buffer.find('\n')) != std::string::npos) {
                std::string msg = pending_buffer.substr(0, pos);
                pending_buffer.erase(0, pos + 1);

                std::cout << "[" << ip << ":" << client_port << "] " << msg << std::endl;

                auto status = client->processMessage(msg);

                std::string resp = "Echo: ";
                if (status.isOk()) {
                    resp = resp + status.getMsg() + "\n";
                } else {
                    resp = resp + status.getErrMsg() + "\n";
                }

                ssize_t sent = send(client_fd, resp.data(), resp.size(), 0);
                if (sent < 0) {
                    std::cerr << "send failed: " << std::strerror(errno) << std::endl;
                    break;
                }
            }
        } else if (n == 0) {
            std::cout << "Client disconnected: " << ip << ":" << client_port << std::endl;
            break;
        } else {
            if (errno == EINTR) {
                continue;
            }
            if (g_stop && errno == EBADF) {
                break;
            }
            std::cerr << "recv failed: " << std::strerror(errno) << std::endl;
            break;
        }
    }

    close(client_fd);
    unregisterClientFd(client_fd);
}

int main() {
    struct sigaction sa {};
    sa.sa_handler = handleSignal;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0;
    sigaction(SIGINT, &sa, nullptr);
    sigaction(SIGTERM, &sa, nullptr);

    int listen_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (listen_fd < 0) {
        std::cerr << "socket failed: " << std::strerror(errno) << std::endl;
        return 1;
    }
    g_listen_fd = listen_fd;

    int opt = 1;
    if (setsockopt(listen_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0) {
        std::cerr << "setsockopt failed: " << std::strerror(errno) << std::endl;
        close(listen_fd);
        return 1;
    }

    sockaddr_in server_addr {};
    server_addr.sin_family = AF_INET;
    server_addr.sin_addr.s_addr = INADDR_ANY;
    server_addr.sin_port = htons(PORT);

    if (bind(listen_fd, reinterpret_cast<sockaddr*>(&server_addr), sizeof(server_addr)) < 0) {
        std::cerr << "bind failed: " << std::strerror(errno) << std::endl;
        close(listen_fd);
        return 1;
    }

    if (listen(listen_fd, BACK_LOG) < 0) {
        std::cerr << "listen failed: " << std::strerror(errno) << std::endl;
        close(listen_fd);
        return 1;
    }

    std::cout << "Server listening on 0.0.0.0:" << PORT << std::endl;

    rocksdb::Options options;
    options.create_if_missing = true;
    rocksdb::Status status = rocksdb::DB::Open(options, ROCKSDB_PAth, &db);
    assert(status.ok());
    std::cout << "Successfully init db. db path: " << ROCKSDB_PAth << std::endl;
    std::cout << "Press Ctrl+C to stop." << std::endl;

    std::vector<std::thread> workers;
    while (!g_stop) {
        sockaddr_in client_addr {};
        socklen_t client_len = sizeof(client_addr);
        int client_fd = accept(listen_fd, reinterpret_cast<sockaddr*>(&client_addr), &client_len);
        if (client_fd < 0) {
            if ((errno == EINTR || errno == EBADF) && g_stop) {
                break;
            }
            if (errno == EINTR) {
                continue;
            }
            std::cerr << "accept failed: " << std::strerror(errno) << std::endl;
            continue;
        }

        registerClientFd(client_fd);
        workers.emplace_back(handleClient, client_fd, client_addr);
    }

    if (listen_fd >= 0) {
        close(listen_fd);
        listen_fd = -1;
        g_listen_fd = -1;
    }

    // Wake up blocking recv() in worker threads so they can exit quickly.
    for (int client_fd : snapshotClientFds()) {
        shutdown(client_fd, SHUT_RDWR);
    }
    for (auto & worker : workers) {
        if (worker.joinable()) {
            worker.join();
        }
    }

    delete db;

    std::cout << "Server stopped." << std::endl;
    return 0;
}
