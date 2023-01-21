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

if __name__ == "__main__":
    argv = sys.argv[1:]
    print(sys.argv)
    print(argv)
    pass
