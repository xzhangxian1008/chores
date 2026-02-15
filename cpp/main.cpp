#include <cassert>
#include <cmath>
#include <cstdlib>
#include <cstring>
#include <leveldb/options.h>
#include <leveldb/slice.h>
#include <rocksdb/slice.h>
#include <sys/types.h>
#include <unistd.h>
#include "util.h"

#include <rocksdb/db.h>

using namespace leveldb;

const std::string rocksdb_path = "/DATA/disk3/xzx/chores/cpp/rocksdb";

void learnRocksDB() {
    rocksdb::DB* db;
    rocksdb::Options options;
    options.create_if_missing = true;
    rocksdb::Status status =
    rocksdb::DB::Open(options, rocksdb_path, &db);
    assert(status.ok());

    std::string value;
    rocksdb::Slice key1("xzx1");
    rocksdb::Status s = db->Get(rocksdb::ReadOptions(), key1, &value);
    assert(s.ok() == rocksdb::Status::Code::kOk);
    if (s.ok()) s = db->Put(rocksdb::WriteOptions(), key1, value);
    if (s.ok()) s = db->Delete(rocksdb::WriteOptions(), key1);

    delete db;
}

int main() {
    learnRocksDB();
}
