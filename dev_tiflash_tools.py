import os
import sys

# Tasks:
#   - start/stop a cluster or start/stop a component of a cluster
#   - copy tiflash binaries to cls cluster's patch directory and do the patching operation.
#   - copy tiflash binaries from cls directory to cls cluster's patch directory
#   - copy tiflash binaries to dev cluster
#   - copy tidb/tikv binaries to cls cluster's patch directory and do the patching operation.(not emergency)
#   - go to tiflash bin/log/conf directory
#   - start/stop tiflash/tidb/tikv

tiflash_tar_name = "tiflash-nightly-linux-amd64.tar.gz"
tiflash_binary_name = "tiflash"
tiflash_proxy_name = "libtiflash_proxy.so"
tiflash_gmssl_name = "libgmssld.so.3"

tiflash_src_build_directory = "/data2/xzx/tiflash/build"
tiflash_src_binary_directory = "%s/dbms/src/Server" % tiflash_src_build_directory
tiflash_src_proxy_directory = "%s/dbms/contrib/tiflash-proxy-cmake/debug" % tiflash_src_build_directory
tiflash_src_gmssl_directory = "%s/contrib/GmSSL/lib" % tiflash_src_build_directory

tiflash_src_binary_binary = "%s/%s" % (tiflash_src_binary_directory, tiflash_binary_name)
tiflash_src_proxy_binary = "%s/%s" % (tiflash_src_proxy_directory, tiflash_proxy_name)
tiflash_src_gmssl_binary = "%s/%s" % (tiflash_src_gmssl_directory, tiflash_gmssl_name)

cls_tiflash_patch_directory = "/data2/xzx/tmp/patches/cls"
cls_tiflash_patch_binary_directory = "/data2/xzx/tmp/patches/cls/tiflash"

dev_tiflash_binary_directory = "/data2/xzx/tiup_deploy/dev/tiflash-6009/bin/tiflash"

tmp_cmd = []

class TiupCmd:
    def __init__(self, argv):
        self.argv = argv

    def execute(self):
        cmd = "tiup "
        if argv[0] == "c":
            cmd += "cluster "
        elif argv[0] == "b":
            cmd += "bench "
        
        
        pass

class CpCmd:
    def __init__(self, argv):
        self.argv = argv

    def execute(self):
        pass

class TmpCmd:
    def __init__(self, argv):
        self.argv = argv

    def execute(self):
        pass

# tu: tiup
#   - c: cluster
#   - b: bench
# cp: copy something
#   - tf: tiflash
#     - d: for dev cluster
#     - c: for cls cluster
class Cmd:
    def __init__(self, argv):
        self.argv = argv

    def execute(self):
        op = self.argv[0]
        if op == "tu":
            cmd = TiupCmd(argv[1:])
        elif op == "cp":
            cmd = CpCmd(argv[1:])
        elif op.isdigit():
            cmd = TmpCmd(argv[1:])


if __name__ == "__main__":
    argv = sys.argv[1:]
    if len(argv) == 0:
        raise Exception("arg num is 0")
    print(sys.argv)
    print(argv)
