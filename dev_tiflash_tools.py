import os
import sys

# Tasks:
#   - copy tidb/tikv binaries to cls cluster's patch directory and do the patching operation.(not emergency)

tiflash_tar_name = "tiflash-nightly-linux-amd64.tar.gz"
tiflash_binary_name = "tiflash"
tiflash_proxy_name = "libtiflash_proxy.so"
tiflash_gmssl_name = "libgmssld.so.3"

mode = "debug" # release or debug

tiflash_src_build_directory = ""
tiflash_src_binary_directory = ""
tiflash_src_proxy_directory = ""
tiflash_src_gmssl_directory = ""

tiflash_src_binary_binary = ""
tiflash_src_proxy_binary = ""
tiflash_src_gmssl_binary = ""

cls_tiflash_patch_directory = "/data2/xzx/tmp/patches/cls"
cls_tiflash_patch_binary_directory = "/data2/xzx/tmp/patches/cls/tiflash"

dev_tiflash_bin_directory = "/data2/xzx/tiup_deploy/dev/tiflash-6009/bin/tiflash"
dev_tiflash_log_directory = "/data2/xzx/tiup_deploy/dev/tiflash-6009/log"
dev_tiflash_conf_directory = "/data2/xzx/tiup_deploy/dev/tiflash-6009/conf"

cls_tiflash_bin_directory = "/data2/xzx/tiup_deploy/cls/tiflash-7003/bin/tiflash"
cls_tiflash_log_directory = "/data2/xzx/tiup_deploy/cls/tiflash-7003/log"
cls_tiflash_conf_directory = "/data2/xzx/tiup_deploy/cls/tiflash-7003/conf"

tmp_cmd = []

def initParam():
    global tiflash_src_build_directory
    global tiflash_src_binary_directory
    global tiflash_src_proxy_directory
    global tiflash_src_gmssl_directory
    global tiflash_src_binary_binary
    global tiflash_src_proxy_binary
    global tiflash_src_gmssl_binary

    if mode == "debug":
        tiflash_src_build_directory = "/data2/xzx/tiflash/build"
    elif mode == "release":
        tiflash_src_build_directory = "/data2/xzx/tiflash/build-release"
    else:
        raise Exception("Invalid mode")

    tiflash_src_binary_directory = "%s/dbms/src/Server" % tiflash_src_build_directory
    tiflash_src_proxy_directory = "%s/contrib/tiflash-proxy-cmake/%s" % (tiflash_src_build_directory, mode)
    tiflash_src_gmssl_directory = "%s/contrib/GmSSL/lib" % tiflash_src_build_directory

    tiflash_src_binary_binary = "%s/%s" % (tiflash_src_binary_directory, tiflash_binary_name)
    tiflash_src_proxy_binary = "%s/%s" % (tiflash_src_proxy_directory, tiflash_proxy_name)
    tiflash_src_gmssl_binary = "%s/%s" % (tiflash_src_gmssl_directory, tiflash_gmssl_name)


def init():
    initParam()


class TiupCmd:
    def __init__(self, argv):
        self.argv = argv

    def execute(self):
        cmd = "tiup"
        if self.argv[0] == "c":
            cmd = "%s cluster" % cmd
            self.__processCluster(cmd, self.argv[1:])
        elif self.argv[0] == "b":
            cmd += "%s bench" % cmd
        else:
            raise Exception("tiup: only support 'c'(cluster) and 'b'(bench)")

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
    # ds: tiup cluster destroy {cluster_name}
    # p: tiup cluster patch {cluster_name} {patch_file} -R {role}
    #    (if role is not provided, it's set to tiflash by default)
    # st: tiup cluster start {cluster_name} [-R {role}]
    # sp: tiup cluster stop {cluster_name} [-R {role}]
    # rt: tiup cluster restart {cluster_name} [-R {role}]
    def __processCluster(self, cmd, argv):
        op = argv[0]
        if op == "l":
            # tiup cluster list
            cmd = "%s list" % cmd
            print(cmd)
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
        elif op == "rt":
            # tiup cluster restart {cluster_name} [-R {role}]
            self.__processClusterRestart(cmd, argv)
        elif op == "ds":
            # tiup cluster destroy {cluster_name}
            self.__processClusterDestroy(cmd, argv)
        else:
            raise Exception("Only support 'l', 'd', 'p', 'st', 'sp'")

    def __processClusterPatch(self, cmd, argv):
        cluster_name = argv[1]
        patch_file = argv[2]
        role = self.__convertFullNameToRoleAbbr("tiflash")
        if len(argv) == 4:
            role = argv[3]
        cmd = "%s patch %s %s -R %s" % (cmd, cluster_name, patch_file, self.__convertRoleAbbrToFullName(role))
        os.system(cmd)

    def __processClusterDisplay(self, cmd, argv):
        self.__processClusterSingleParm(cmd, "display", argv[1])

    def __processClusterDestroy(self, cmd, argv):
        self.__processClusterSingleParm(cmd, "destroy", argv[1])

    def __processClusterSingleParm(self, cmd, op, arg):
        cmd = "%s %s %s" % (cmd, op, arg)
        print(cmd)
        os.system(cmd)

    def __processClusterStart(self, cmd, argv):
        self.__processClusterStartOrStopOrRestart(cmd, "start", argv)

    def __processClusterStop(self, cmd, argv):
        self.__processClusterStartOrStopOrRestart(cmd, "stop", argv)

    def __processClusterRestart(self, cmd, argv):
        self.__processClusterStartOrStopOrRestart(cmd, "restart", argv)

    def __processClusterStartOrStopOrRestart(self, cmd, op, argv):
        cluster_name = argv[1]
        if len(argv) == 2:
            cmd = "%s %s %s" % (cmd, op, cluster_name)
        elif len(argv) == 3:
            cmd = "%s %s %s -R %s" % (cmd, op, cluster_name, self.__convertRoleAbbrToFullName(argv[2]))
        else:
            raise Exception("Invalid argv length")
        os.system(cmd)

# df: cluster name: dev, role: tiflash
# cf: cluster name: cls, role: tiflash
class CpCmd:
    def __init__(self, argv):
        self.argv = argv

    def execute(self):
        arg = self.argv[0]
        if arg == "df":
            df_cmd = "cp %s %s && cp %s %s" % (tiflash_src_binary_binary, dev_tiflash_bin_directory, tiflash_src_proxy_binary, dev_tiflash_bin_directory)
            print(df_cmd)
            os.system(df_cmd)
        elif arg == "cf":
            cf_cmd = "cp %s %s && cp %s %s" % (tiflash_src_binary_binary, cls_tiflash_patch_binary_directory, tiflash_src_proxy_binary, cls_tiflash_patch_binary_directory)
            print(cf_cmd)
            os.system(cf_cmd)
        else:
            raise Exception("Invalid arg for CpCmd")

class TmpCmd:
    def __init__(self, argv):
        self.argv = argv

    def execute(self):
        arg = self.argv[0]
        idx = int(arg)
        if idx >= len(tmp_cmd):
            raise Exception("Can't find this temporary cmd")
        os.system(tmp_cmd[idx])

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
        global mode

        op = self.argv[0]
        if op == "tu":
            cmd = TiupCmd(argv[1:])
            cmd.execute()
        elif op == "cpd":
            cmd = CpCmd(argv[1:])
            if mode != "debug":
                raise Exception("mode != debug")
            cmd.execute()
        elif op == "cpr":
            cmd = CpCmd(argv[1:])
            if mode != "release":
                raise Exception("mode != release")
            cmd.execute()
        elif op.isdigit():
            cmd = TmpCmd(argv)
            cmd.execute()
        else:
            raise Exception("Only support 'tu', 'cp', digital")


if __name__ == "__main__":
    argv = sys.argv[1:]
    if len(argv) == 0:
        raise Exception("arg num is 0")
    init()
    cmd = Cmd(argv)
    cmd.execute()
