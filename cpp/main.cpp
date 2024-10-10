#include "util.h"
#include "bench.h"
#include <leveldb/db.h>
#include <leveldb/options.h>
#include <stdexcept>

const std::string leveldb_data_file = "/DATA/disk3/xzx/tmp/leveldb_data/testdb";

int main() {
    // leveldb::DB* db;
    // leveldb::Options options;
    // options.create_if_missing = true;
    // leveldb::Status status = leveldb::DB::Open(options, leveldb_data_file, &db);
    // assert(status.ok());
    // leveldb::DestroyDB(leveldb_data_file, leveldb::Options());
    std::cout << "1" << std::ends << "1";
}
