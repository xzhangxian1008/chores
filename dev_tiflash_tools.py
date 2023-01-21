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
        if self.argv[0] == "c":
            cmd += "cluster"
            self.__processCluster(cmd, self.argv[1:])
        elif self.argv[0] == "b":
            cmd += "bench"

    def __convertRoleAbbrToFullName(self, abbr_role):
        if abbr_role.lower() == "tf":
            return "tiflash"
        elif abbr_role.lower() == "td":
            return "tidb"
        elif abbr_role.lower() == "tk":
            return "tikv"
        else:
            raise Exception("Invalid abbreviation")

    def __convertFullNameToRoleAbbr(self, full_name):
        if full_name.lower() == "tiflash":
            return "tf"
        elif full_name.lower() == "tikv":
            return "tk"
        elif full_name.lower() == "tidb":
            return "td"
        else:
            raise Exception("Invalid role abbrevation")

    # l: tiup cluster list
    # d: tiup cluster display {cluster_name}
    # p: tiup cluster patch {cluster_name} {patch_file} -R {role}
    #    (if role is not provided, it's set to tiflash by default)
    # st: tiup cluster start {cluster_name} [-R {role}]
    # sp: tiup cluster stop {cluster_name} [-R {role}]
    def __processCluster(self, cmd, argv):
        op = argv[0]
        if op == "l":
            # tiup cluster list
            cmd += "list"
            os.system(cmd)
        elif op == "d":
            # tiup cluster display {cluster_name}
            self.__processClusterDisplay(cmd, argv)
        elif op == "p":
            # tiup cluster patch {cluster_name} {patch_file} -R {role}
            self.__processClusterPatch(cmd, argv)
        elif op == "st":
            # tiup cluster start {cluster_name} [-R {role}]
            self.__processClusterStart(cmd, argv)
        elif op == "sp":
            # tiup cluster stop {cluster_name} [-R {role}]
            self.__processClusterStop(cmd, argv)

    def __processClusterPatch(self, cmd, argv):
        cluster_name = argv[1]
        patch_file = argv[2]
        role = self.__convertFullNameToRoleAbbr("tiflash")
        if len(argv) == 4:
            role = argv[3]
        cmd = "%s patch %s %s -R %s" % (cmd, cluster_name, patch_file, self.__convertRoleAbbrToFullName(role))
        os.system(cmd)

    def __processClusterDisplay(self, cmd, argv):
        cmd = "%s display %s" % (cmd, argv[1])
        os.system(cmd)

    def __processClusterStart(self, cmd, argv):
        self.__processClusterStartOrStop(cmd, "start", argv)

    def __processClusterStop(self, cmd, argv):
        self.__processClusterStartOrStop(cmd, "stop", argv)

    def __processClusterStartOrStop(self, cmd, op, argv):
        cluster_name = argv[1]
        if len(argv) == 2:
            cmd = "%s %s %s" % (cmd, op, cluster_name)
        elif len(argv) == 3:
            cmd = "%s %s %s -R %s" % (cmd, op, cluster_name, self.__convertRoleAbbrToFullName(argv[2]))
        else:
            raise Exception("Invalid argv length")
        print(cmd)
        # os.system(cmd)


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
            cmd.execute()
        elif op == "cp":
            cmd = CpCmd(argv[1:])
            cmd.execute()
        elif op.isdigit():
            cmd = TmpCmd(argv[1:])
            cmd.execute()


if __name__ == "__main__":
    argv = sys.argv[1:]
    if len(argv) == 0:
        raise Exception("arg num is 0")
    # print(sys.argv)
    # print(argv)
    cmd = Cmd(argv)
    cmd.execute()
