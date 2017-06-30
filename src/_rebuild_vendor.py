# -*- coding:utf-8 -*-
import os
import glob
import subprocess
import sys

GOPATH = ""
PATH = ""
GO_EXE_PATH="go"
def rebuild_vendor_packages(root_dir):
    for dir in os.listdir(root_dir):
        full_dir = os.path.join(root_dir, dir)
        if os.path.isdir(full_dir):
            # 如果是碰到测试，example目录，则直接跳过
            if full_dir.find("example") != -1 or full_dir.find("test") != -1:
                continue

            files = glob.glob("%s/*.go" % full_dir)
            if len(files) > 0:
                print "Install golang package: %s" % full_dir
                subprocess.Popen([GO_EXE_PATH, "install", full_dir], env={'GOPATH': GOPATH, 'PATH':PATH}).wait()
            else:
                # 如果碰到空目录，则继续遍历
                rebuild_vendor_packages(full_dir)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print "No go path is sepecified"
        exit(0)
    GO_EXE_PATH = sys.argv[1]

    GOPATH = os.environ.get("GOPATH", "")
    if not GOPATH:
        print "GOPATH is not set"
        exit(0)
    PATH = os.environ.get("PATH", "")
    rebuild_vendor_packages("vendor")
